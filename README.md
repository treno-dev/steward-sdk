# Steward SDK

Everything you need to write a Steward plugin: the contract, SDKs for Go, JavaScript, TypeScript and
Python, and the templates `steward plugin init` starts a project from.

## Steward, and the systems it manages

Steward gives a company one standard way to manage everything its teams use across many systems. It
covers three things, and treats them all the same way:

- **Access:** who can use what, and with which role. Give Amina write access to the payments
  repositories for a week; remove everything she holds when she leaves.
- **Resources:** provisioning and deprovisioning the things teams work with. Create a repository, a
  storage bucket or a database for a new project; remove them when the project ends.
- **Applications:** creating, deploying and deleting the things teams run, with their settings and
  secrets. Deploy this web app from that branch with these environment variables.

Instead of an admin clicking through a dozen consoles, each with its own way of doing these things,
people ask Steward. Steward records who asked, routes the request for approval, carries it out, keeps
track of what now exists, and undoes it when it expires or is no longer needed.

None of that work happens inside Steward. It happens in the **external systems** the company already
uses: GitHub, AWS, Google Cloud, Jira, Cloudflare, a database server, an internal admin panel. Each of
these has its own vocabulary and its own API:

| System             | Resources it provisions      | Applications it runs       | Access it grants                                |
| ------------------ | ---------------------------- | -------------------------- | ----------------------------------------------- |
| GitHub             | repositories, teams          |                            | `read`, `write`, `admin` on a repository        |
| AWS                | buckets, databases, accounts | containers, functions      | policies made of actions such as `s3:GetObject` |
| A hosting platform | sites, databases             | web apps deployed from git | team membership                                 |
| Jira               | projects                     |                            | project roles such as `Developer`               |

Steward needs to do the same few things in all of them: create things, remove things, deploy things,
let someone in, change what they can do, take it away. But "create a bucket" or "give Amina write
access" means a completely different API call in each system.

## What a plugin is

A **plugin** is a small program that teaches Steward one external system. It is the translator between
Steward's questions and that system's API. The GitHub plugin, for example:

- **declares what it needs to connect:** the GitHub organization and an access token. Steward shows
  these as a form when someone adds the plugin. A company can add the same plugin several times, such
  as for two GitHub organizations, each with its own settings and credentials.
- **declares what it manages:** kinds of **resources**, such as `repository`, each with the settings it
  takes to create one (visibility, description) and what it reports back once it exists (its URL).
- **declares what can be granted:** the roles and permissions GitHub offers, such as `read`, `write`
  and `admin` on a repository.
- **carries out Steward's calls:** create the repository `payments-api`, give `amina` the role `write`
  on it, take it away again, tell me what `amina` holds right now.

Those calls fall into three kinds of work. A plugin supports whichever fit its system:

- **Resource provisioning:** create and remove resources, and report what they produced, such as a URL
  or an id. Optionally, list resources that already exist so they can be brought under Steward.
- **Application management:** create, deploy and destroy applications, which are run from a source
  (a GitHub repository, a container image, an S3 archive or a path) and have variables, such as a web app
  and its environment.
- **Access:** grant, revoke and report someone's roles, on the system as a whole (membership of the
  GitHub organization) or on one resource (a role on one repository).

A plugin supports whichever it implements, for the system as a whole and for each kind of resource,
and Steward offers only that. The list is designed to grow: new kinds of work are added to the
contract beside these without breaking plugins that already exist.

### Everything is extensible through plugins

Steward has no built-in list of systems, resources or applications. Everything it can create, deploy
or grant comes from a plugin, so anything a program can do, Steward can manage:

- **Bring your existing infrastructure as code.** A plugin does not have to call a system's API
  directly. Its `provision` can run the infrastructure-as-code, templates or scripts your team already
  maintains, and return their outputs. The code stays yours; Steward puts requests, approvals,
  ownership, expiry and an audit trail around it.
- **Define your own kinds of resources.** A kind is whatever makes sense to your teams: not only "a
  bucket", but "a production-ready service" that creates a repository, a database and a pipeline
  together, with the inputs your platform team chooses to expose.
- **Wrap internal systems.** An admin panel, a homegrown deployment tool or a legacy database can get
  a plugin like any public service, and is then managed the same way as GitHub or AWS.
- **Keep it private.** A plugin can live in your own repository and run only in your own runners. It
  never has to be published.

The plugin never decides anything. Steward decides what should exist and who should have what, keeps
the record of it, and works out what changed. The plugin is told what to do, does it, and reports the result.

## Why plugins

- **No single team can cover every system.** There are thousands, each with its own API, and each
  changes on its own schedule. A small, stable contract lets anyone add one, whether that is us, the
  community, or a company writing a private plugin for its own internal system, without any change to
  Steward.
- **Steward does not run other people's code, and does not hold their credentials.** A plugin runs in a
  **runner**, a process the company operates in its own environment, next to the systems and secrets it
  needs. Steward sends work to the runner; the runner starts the plugin, passes it the credentials for
  that one call, and sends back the result.
- **Every system looks the same to Steward.** Whatever the plugin talks to, Steward asks the same
  questions and gets the same kinds of answers. That is what lets approvals, time-limited access,
  offboarding, audit history and the interface work identically for GitHub, AWS, or a system Steward
  has never heard of.

## How it works

1. **A plugin is a program.** You write it with an SDK and build or run it like any other. Steward
   never runs plugin code itself: a runner, which people operate in their own environment, starts the
   plugin as a subprocess when it needs it.
2. **The runner and the plugin speak gRPC.** The plugin listens on a unix socket that only the runner's
   user can open (a local port on Windows) and prints one line, the handshake, so the runner knows where
   to connect. Then the runner describes the plugin, checks its health and calls it.
3. **The contract is a protobuf file.** [`proto/steward/plugin/v1/plugin.proto`](proto/steward/plugin/v1/plugin.proto)
   defines every message and call. The SDKs are generated from it, so they all behave the same way.
   Within `v1` the contract only grows.
4. **The SDK hides the protocol.** You declare the plugin and its kinds of resources, and write
   one function per call. The SDK serves gRPC, prints the handshake, answers the health check, routes
   each call to your function by kind, and answers anything you did not write with `UNIMPLEMENTED`.

### Any language works

The SDKs are a convenience, not a requirement. The runner speaks the protocol of
[`hashicorp/go-plugin`](https://github.com/hashicorp/go-plugin) (a subprocess, a handshake line, gRPC),
so a plugin can be written in any language that can serve gRPC. A program is a Steward plugin if it:

1. **prints the handshake line** on stdout once it is listening, `1|1|unix|<socket path>|grpc` (or
   `1|1|tcp|127.0.0.1:<port>|grpc`);
2. **serves the services of the contract**: `PluginService` always, and `ResourceService` and
   `ApplicationService` for what it supports, generated from
   [the proto](proto/steward/plugin/v1/plugin.proto) with `protoc` or `buf` in your language; and
3. **serves the standard gRPC health service**, reporting `SERVING` for the service named `plugin`.

That is the whole obligation. Nothing else is needed, and the plugin never has to import Steward code.

A runner also sets some environment, which a plugin should honour:

| Variable | What the plugin does |
|---|---|
| `STEWARD_PLUGIN` | The magic cookie. Without it, the plugin was started by hand, so it says so on standard error and exits. The value is in [`go/plugin/handshake.go`](go/plugin/handshake.go). |
| `PLUGIN_UNIX_SOCKET_DIR` | When set, listen on a unix socket created in this directory, which only the runner's user can open, and announce it in the handshake. Without it, listen on a loopback port. |

A plugin can also serve `plugin.GRPCController/Shutdown`, a call with empty messages that go-plugin makes
to ask it to stop. Without it the runner waits two seconds and then kills the plugin.

What the SDK adds is what makes writing one easy: it does those three things for you, turns the
`google.protobuf.Struct` values into plain objects, routes each call to your function by kind, builds
the description from what you declared, fills in what a handler leaves out, and answers what you did
not write with `UNIMPLEMENTED`. So the SDKs for JavaScript, TypeScript, Python and Go are the easy
road, and a plugin in Rust, Java or anything else is the same contract without that help.

### The pieces of a plugin

- **An integration**: an instance of the plugin, with the inputs and secrets it is set up with. The plugin declares its **inputs**, like variables with a name
  and a type, and its **secrets**, the credentials it needs, with a name and a description. The runner
  passes secrets to each call separately from the inputs so the plugin never has to store them.
- **Resources**: the things the tool holds, each declared once under a **kind** (`"repository"`) with
  its own inputs and **outputs** (what comes back once it exists, such as a URL). A resource is
  identified by its kind and name.
- **Access**: roles and permissions that can be granted on the plugin as a whole, or on one resource.
  They are two separate lists: the **roles** the tool defines itself, and the **permissions** Steward
  builds its own roles from. Declaring either says access is supported.
- **Applications** (optional): things that are deployed from a source and have open-ended variables,
  as opposed to resources, which are provisioned.

Steward offers what you implement. With `provision` and `deprovision` a kind can be created and
removed, with `list` it can be discovered, and with roles or permissions and the access functions it
can have access granted. A call for a function you did not write answers `UNIMPLEMENTED`, and Steward
treats that as not offered.

Calls that change something are **idempotent** and return the **result** (the outputs, the role as the
tool applied it), not a report of what changed. Steward works out the difference itself.

## A plugin, in JavaScript

```sh
steward plugin init my-plugin --language js    # or ts, python, go
cd my-plugin
npm install
steward plugin validate node src/plugin.js
```

This is the heart of what that generates, shortened. The full file, with every call and its comments,
is [`templates/js/src/plugin.js.tmpl`](templates/js/src/plugin.js.tmpl).

```js
import { createPlugin } from "@treno-dev/steward-sdk/plugin";

// The plugin: what it takes to connect to the tool.
const plugin = createPlugin({
  name: "my-plugin",
  version: "0.1.0",
  title: "My plugin",

  inputs: [{ name: "base_url", label: "Base URL", type: "string", required: true }],

  secrets: [
    { name: "API_TOKEN", description: "The token to call the API with.", required: true },
  ],

  // Return an error for each input or secret the user can fix.
  async validate({ integration }) {
    const errors = [];

    if (!integration.secrets.API_TOKEN) {
      errors.push({ field: "API_TOKEN", message: "An API token is required." });
    }

    return { errors };
  },

  // Access to the tool as a whole, such as membership of an organization.
  roles: [{ name: "member", title: "Member" }],

  async grantAccess({ integration, identity, role }) {
    return { role };
  },

  async revokeAccess({ integration, identity }) {
    return {};
  },

  async getAccess({ integration, identity }) {
    return { roles: [] };
  },
});

// A kind of resource it manages.
plugin.resource({
  kind: "item",
  title: "Item",
  outputs: [{ name: "url", label: "URL", type: "string" }],
  permissions: [
    { name: "view", title: "View" },
    { name: "edit", title: "Edit" },
  ],
  roles: [
    { name: "viewer", title: "Viewer" },
    { name: "editor", title: "Editor" },
  ],

  // Create the resource. Idempotent. Return what it produced.
  async provision({ integration, name }) {
    return { outputs: { url: `${integration.inputs.base_url}/items/${name}` } };
  },

  async deprovision({ resource }) {
    return {};
  },

  // Access to one resource: give an identity a role on it. The role is always set, and `permissions`
  // lists the pieces of a role Steward composed. Return what the tool applied.
  async grantAccess({ integration, resource, identity, role, permissions }) {
    return { role, permissions };
  },

  async revokeAccess({ identity, role, permissions }) {
    return {};
  },

  async getAccess({ identity }) {
    return { roles: [], permissions: [], pending: false };
  },
});

plugin.serve();
```

A plugin is started by a runner, which sets what it needs in the environment. Started by hand it says
so and exits, so try it with `steward plugin validate node src/plugin.js` (see below). The runner reads
one line from it, the handshake, and then calls it:

```
1|1|unix|/tmp/steward-2871798330/plugin-4855f4bd|grpc
```

Handlers receive plain objects and may leave out any field of the response they have nothing to say
about. To fail a call, throw `{ code, message }` with a gRPC status code. Write logs to standard error,
which the runner collects.

The other languages follow the same shape, in their own idiom:

| Language               | Install                                      | Declare                                                 | A handler                                        |
| ---------------------- | -------------------------------------------- | ------------------------------------------------------- | ------------------------------------------------ |
| JavaScript, TypeScript | `npm install @treno-dev/steward-sdk`         | `createPlugin({...})`, `plugin.resource({ kind, ... })` | `async provision({ integration, name }) { ... }`      |
| Python                 | `pip install treno-dev-steward-sdk`          | `create_plugin(...)`, `plugin.resource(kind=..., ...)`  | `async def provision(request): ...`              |
| Go                     | `go get github.com/treno-dev/steward-sdk/go` | `plugin.New(...)`, `p.Resource("kind", ...)`            | `func(ctx, *Request) (*Response, error)`         |

Each is one SDK package per language, and the plugin code is its `plugin` part: `@treno-dev/steward-sdk/plugin`
in JavaScript, `steward_sdk.plugin` in Python and `…/go/plugin` in Go. Other parts of the SDK will sit
beside it, under the same package name.

## What is here

|                           |                                                                                                    |
| ------------------------- | -------------------------------------------------------------------------------------------------- |
| [`proto/`](proto)         | The contract, with `buf` lint and breaking-change checks.                                          |
| [`js/`](js)               | `@treno-dev/steward-sdk` for JavaScript and TypeScript.                                            |
| [`python/`](python)       | `treno-dev-steward-sdk` for Python, imported as `steward_sdk`.                                     |
| [`go/`](go)               | The Go module, `github.com/treno-dev/steward-sdk/go`.                                              |
| [`templates/`](templates) | One template per language (`js`, `ts`, `python`, `go`), the projects `steward plugin init` writes. |

Each SDK has a README with everything a plugin can declare: [JavaScript](js/README.md),
[Python](python/README.md), [Go](go/README.md).

## Templates

`steward plugin init` does not carry its examples. It downloads the templates of an SDK release and
checks them against their checksum, so a new project always matches the SDK it depends on.

```sh
steward plugin init my-plugin --language js
```

The project is created in a folder named after the plugin, in the current directory. The name takes
lowercase letters, numbers and dashes. While it works, the command prints a line for each step, such as
getting the templates and writing the project.

| Flag               |                                                                                                                                                                                                                                                                                                                                      |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `--language`, `-l` | Required: `js`, `ts`, `python` or `go`.                                                                                                                                                                                                                                                                                              |
| `--path`, `-p`     | The directory to create the plugin's folder in. The default is the current directory. A folder of the plugin's name that already exists must be empty.                                                                                                                                                                               |
| `--sdk-version`    | The SDK release to take the templates from. The default is the latest release; pre-releases such as `0.1.0-rc.1` have to be asked for.                                                                                                                                                                                               |
| `--template`       | Templates from somewhere else, instead of the release: anything [go-getter](https://github.com/hashicorp/go-getter) understands, such as a URL, a path or a git repository, holding `templates/<language>`. Add `?checksum=sha256:…` to verify it. With `--template`, `--sdk-version` says which SDK version the project depends on. |
| `--sdk-path`       | A checkout of this repository, for working on an SDK and its template together. The project depends on the SDK in that checkout.                                                                                                                                                                                                     |

```sh
steward plugin init my-plugin -l python --path ~/code/plugins
steward plugin init my-plugin -l js --sdk-version 0.1.0-rc.1
steward plugin init my-plugin -l js --sdk-path /path/to/steward-sdk
```

## Check and run a plugin

Two commands start a plugin the way a runner would and talk to it, whatever language it is written in.
Give either one the command that starts the plugin. `steward plugin validate` checks it and never changes
anything. `steward plugin run` invokes it, for real.

### Validate

```sh
steward plugin validate node src/plugin.js
steward plugin validate .venv/bin/python src/plugin.py
steward plugin validate go run .
```

```
✓ handshake       unix /tmp/steward-2871798330/plugin-4855f4bd, contract version 1
✓ health          the service "plugin" is SERVING
✓ describe        my-plugin 0.1.0: 1 resource kind(s) (item), 0 application kind(s)
✓ inputs          every input and output has a name and a known type
✓ kinds           kinds are named and unique
✓ roles           roles and permissions are named and unique
✓ validate        answered, and the inputs are good
✓ plugin access   not declared, and GetAccess is UNIMPLEMENTED
✓ item access     declared, and GetAccess is served
✓ item discovery  List is not offered
```

It only reads: it checks the handshake, the health service, what the plugin describes about itself, that
`Validate` answers, that the plugin and each kind serve access exactly when they declare roles or
permissions, and which kinds offer discovery. It never creates or changes anything, so it is safe to
run against real credentials.

### Run

`steward plugin run` invokes the plugin with a context file saying what to use. It gives the identity a
role on the plugin, then for each resource in the context creates it, gives the identity a role on
it and takes the role away, and removes it. It removes what it created even when a step fails.

It does all of this for real, in whatever the plugin's credentials reach, so point it at a test account.
It lists what it is about to do and asks before it starts, unless you pass `--yes`, which a script or CI
needs because there is nobody to ask.

```sh
steward plugin run --context context.json node src/plugin.js
steward plugin run --yes --context context.json go run .
```

To work on one call at a time, name it with `--call`. It makes only that call, once, on every resource in
the context, prints what the plugin answered, and cleans nothing up, since leaving what a call made in
place is the point. The calls are `provision`, `grant-access`, `get-access`, `revoke-access` and
`deprovision`. Repeat the flag, or separate names with commas, to make several, and they run in lifecycle
order. The access calls also run against the plugin when it declares roles or permissions.

```sh
steward plugin run --context context.json --call provision node src/plugin.js
✓ item/demo provision  {"outputs":{"url":"https://example.test/items/demo"}}

steward plugin run --context context.json --call grant-access --call get-access node src/plugin.js
steward plugin run --context context.json --call deprovision node src/plugin.js
```

A call a kind doesn't offer, such as `grant-access` on a kind without access, is skipped. Responses are
printed as they come, so outputs marked sensitive show in your terminal as well.

### The context file

```json
{
  "integration": {
    "inputs": { "base_url": "https://example.test" },
    "secrets": { "API_TOKEN": "..." }
  },
  "identity": { "external_id": "someone" },
  "role": { "name": "viewer", "permissions": [{ "name": "view" }] },
  "resources": [
    { "kind": "item", "name": "demo", "inputs": { "description": "a test item" } },
    { "kind": "item", "name": "second" }
  ]
}
```

- **`integration`** holds the values the plugin is invoked with, as Steward would supply them: its inputs and
  its secrets.
- **`resources`** is a list of the requests that create resources. Each has a `kind` the plugin declares, a
  `name`, and the `inputs` and `secrets` of that kind. They run in the order listed, and you can list
  several of the same kind. Each `kind` and `name` pair can appear once.
- **`identity`** and **`role`** are who is given which role, on the plugin and on each resource. The role
  is always named. Its `permissions` are only for a role Steward composed from the plugin's permissions.

`run` needs the file. `validate` can take it too, with `--context`, to give the plugin the integration the read-only
calls are made with. Both commands exit with a failure when a check fails, so they can run in CI, and
`--verbose` shows what the plugin writes to standard error.

## Document a plugin

`steward plugin docs` starts a plugin, asks it what it describes, and writes markdown reference pages from
the answer: one for the plugin, one for each kind of resource and one for each kind of application, with
their inputs, secrets, outputs, roles and permissions. The pages come from the plugin, so they cannot
differ from it.

```sh
steward plugin docs node src/plugin.js
steward plugin docs --out site python src/plugin.py
```

The pages are written to `docs`, or to `--out`, in the plugin's folder (the current directory, or the one
given with `--cwd`). Commit them, so the registry reads them with the plugin.

Each page can be a template, so you decide where the generated parts go. A page without one uses a
default layout:

| File                                      |                                                    |
| ----------------------------------------- | -------------------------------------------------- |
| `templates/index.md.tmpl`                 | The plugin page.                                   |
| `templates/resources/<kind>.md.tmpl`      | A resource kind.                                   |
| `templates/applications/<kind>.md.tmpl`   | An application kind.                               |
| `examples/<kind>.json`                    | A sample resource: `{ "kind", "name", "inputs" }`. |

A file ending in `.md.tmpl` is a Go template. Place the generated parts with `{{ .Title }}`,
`{{ .Description }}`, `{{ .Example }}`, `{{ .Inputs }}`, `{{ .Secrets }}`, `{{ .Outputs }}`,
`{{ .Sources }}` (applications), `{{ .Roles }}` and `{{ .Permissions }}`, or all the tables at once with
`{{ .Reference }}`. The plugin page also has `{{ .Resources }}` and `{{ .Applications }}`, the lists of
links. A part with nothing to show is empty. The same name ending in `.md`, such as
`templates/index.md`, is used as it is.

```
# {{ .Title }}

Start here: this plugin manages items.

{{ .Secrets }}
{{ .Resources }}
```

Every file is optional. Write a `description` on every input, secret and output, because that is what the
tables show.

## Developing

The SDKs are generated from the contract with `buf` (remote plugins, so it needs network access).
The JavaScript and Python builds generate on demand and do not commit the result. The Go module
commits `go/gen`, because Go users install a module as it is.

```sh
cd proto && buf lint && buf build && buf format -d    # check the contract
cd js && npm install && buf generate && npm run build  # JavaScript and TypeScript
cd python && buf generate                              # Python
cd go && buf generate && go vet ./...                  # Go
```

Changing the contract: add, never renumber or remove (`buf breaking` fails any change that does).
Regenerate every SDK, and keep each template in step, because the READMEs embed their examples.
