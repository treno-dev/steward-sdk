# Steward SDK for Go

The Steward SDK for Go. Today it holds the part for writing Steward plugins, in the package
`github.com/treno-dev/steward-sdk/go/plugin`. A plugin is one integration: it declares what it needs to
connect to a system and what it manages there, and implements the calls Steward makes. The SDK handles
everything else: the gRPC services, the handshake with the runner, the health check, and routing each
call to your handler by kind.

```sh
go get github.com/treno-dev/steward-sdk/go
```

```go
import "github.com/treno-dev/steward-sdk/go/plugin"
```

Requires Go 1.25 or newer.

## Start a plugin

```sh
steward plugin init my-plugin --language go
cd my-plugin
go mod tidy
steward plugin validate go run .
```

`steward plugin init` takes its templates from the SDK release, so the project always matches the SDK
it depends on. To work on the SDK and its templates together, point it at a local checkout with
`--sdk-path /path/to/steward-sdk`.

The generated plugin, with a resource that can be provisioned and given access to, is
[`../templates/go/main.go.tmpl`](../templates/go/main.go.tmpl). A plugin needs one
`plugin.New(...)`, any number of `Resource(...)` and `Application(...)` declarations, and a final
`Serve()`. If it needs more than one tool, for example both Cloudflare and AWS, take the credentials for
each as inputs.

```go
integration := plugin.New(plugin.Options{Name: "github", Version: "0.1.0", Inputs: inputs, Validate: validate})

integration.Resource("repository", plugin.ResourceOptions{
	Provision:   provision,
	Deprovision: deprovision,
})

integration.Serve()
```

## What you declare

**The integration**, in `plugin.Options`:

| Field | |
|---|---|
| `Name`, `Version` | Required. |
| `Title`, `Description` | Shown in Steward. |
| `Inputs` | What is required to create an integration, credentials included. |
| `Validate` | Optional. Checks the credentials, which Steward cannot. Returns `ValidationError`s the user can fix. Without it, every integration is accepted. |
| `Roles`, `Permissions`, `GrantAccess`, `RevokeAccess`, `GetAccess` | Access to the integration as a whole, such as membership of an organization. |

**A resource**, with `integration.Resource("kind", plugin.ResourceOptions{...})`, and **an application**,
with `integration.Application("kind", plugin.ApplicationOptions{...})`, are each declared under a kind that
is unique within the plugin, such as `"repository"`, and take a `Title`, a `Description`, `Inputs` and
`Outputs`. Resources also take `Roles` and `Permissions`. Applications take `Sources`, the types they can
be deployed from: `"github"`, `"registry"`, `"s3"` or `"raw"`. The `Source` of an application arrives as
`Type`, `Config` and `Ref`, where `Config` holds the settings of that type: `github` `{owner, name,
branch?}`, `registry` `{image, tag?}`, `s3` `{bucket, key}` and `raw` `{path}`. `Ref` pins a commit, an
image digest or an object version, and is empty for the newest.

**Inputs and outputs** are declared like variables:

```go
&plugin.InputDefinition{Name: "visibility", Label: "Visibility", Type: plugin.TypeSelect, Options: []string{"private", "public"}}
```

`Type` is one of `TypeString`, `TypeNumber`, `TypeBoolean`, `TypeSelect`, `TypeList` (of strings) or
`TypeMap`. Mark a credential `Sensitive: true` and it is never shown back once set.

## Access

Access exists at two levels, each with the same three handlers, `GrantAccess`, `RevokeAccess` and
`GetAccess`:

- **The integration**, in `plugin.Options`: membership of the tool as a whole, such as an organization
  or a site. Steward grants this first, then access to the resources.
- **A resource kind**, in `plugin.ResourceOptions`: a role on one resource, such as a repository.

What you declare with `Roles` (and, for tools that expose them, `Permissions`) is what Steward offers
for assignment. A role has a `Name` and the `Permissions` it contains, and a permission is a `Name`
with an optional `Level`. Steward can also build its own roles from the permissions you declare.

Handlers receive the `Identity` (`ExternalId`, `Name`, `Attrs`) and the `Role` to apply, and a resource
call also gets the `Resource` (`Kind`, `Name`). They return:

- `GrantAccess`: the `Role` as the tool applied it, which may differ from the one requested, an `Id`
  for the grant if the tool has one (Steward sends it back on revoke), and `Pending: true` while the
  person has to act first, such as accepting an invitation.
- `RevokeAccess`: nothing. It succeeds if the identity did not have the role.
- `GetAccess`: the `Roles` the identity holds now, and whether a grant is still `Pending`.

Steward works out overlap between roles before it asks you to revoke one, so you can remove the role
you are given in full.

## Capabilities follow from your handlers

You never list capabilities. The SDK reads them from the handlers you set:

| You set | The kind can |
|---|---|
| `Provision`, `Deprovision` | be created and removed (provisioning) |
| `List` | be discovered (optional: Steward keeps track of what it creates, so `List` is only for finding resources that already exist) |
| `GrantAccess`, `RevokeAccess`, `GetAccess` | have access granted (resources and the integration) |
| `Create`, `Delete`, `Deploy`, `SetVariables`, `List` | be run as an application |

A call for something you did not set fails with `UNIMPLEMENTED`.

## Handlers

A handler is `func(context.Context, *Request) (*Response, error)`, with the request and response named
for the entity and the call: `ResourceProvisionRequest`, `ResourceProvisionResponse`. They are the
generated contract messages, re-exported by the package so a plugin imports one thing.

- **Getters are nil-safe.** Use `request.GetIntegration().GetSecrets()["token"]`: anything the runner
  did not send reads as empty.
- **Inputs.** `Integration.Inputs` and `Inputs` hold the values that are not sensitive; call `.AsMap()`
  on one for a plain `map[string]any` (numbers arrive as `float64`). Sensitive values arrive in
  `Integration.Secrets` (and a resource's or application's `Secrets`) as a `map[string]string`. Never
  store them.
- **Open-ended values in a response**, such as `Outputs`, are built with `plugin.Struct(map[string]any{...})`,
  and an input's `Default` with `plugin.Value(...)`. They take what JSON can hold; anything else
  panics, which the SDK reports as an `INTERNAL` error with the stack on stderr, as it does for any
  panic in a handler.
- **Idempotent, and no report of changes.** Make calls that change something idempotent: creating what
  exists, or removing what is gone, simply succeeds. Return the result of the change (the `Outputs`, the
  applied `Role`), not a description of what changed: Steward keeps the state and works out the
  difference itself.
- **Resources and identities.** A resource is identified by its `Kind` and `Name`, which Steward stores
  and sends on every later call. `Provision` receives them with the `Inputs` and returns only the
  `Outputs`, such as a URL or an id the tool assigned.
- **Variables.** An application's variables each carry `Sensitive`, `Sealed` and `Locked`. Refuse to
  change or remove a locked variable.

## Errors

Return a gRPC status to report a specific failure:

```go
return nil, status.Error(codes.FailedPrecondition, "APP_KEY is locked and cannot be removed")
```

Common codes: `InvalidArgument`, `NotFound`, `FailedPrecondition`, `Unimplemented`. Any other error is
reported as `INTERNAL`. A call for an unknown kind is answered with `NotFound` by the SDK.

## Logging

Write logs to standard error, which the runner collects. A plugin must be started by a runner, which sets
what `plugin.Handshake` describes: started by hand it says so and exits, so use `steward plugin validate`.

## Developing the SDK

The messages and gRPC services in `gen/` are generated from `../proto` with `buf` (`buf.gen.yaml`,
using buf's remote plugins, so it needs network access). Unlike the other SDKs the generated code **is
committed**, because Go consumers install the module as it is, with no build step. Regenerate it after
the contract changes:

```sh
buf generate
go vet ./... && go build ./...
```
