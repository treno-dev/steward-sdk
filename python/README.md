# treno-dev-steward-sdk

The Steward SDK for Python. Today it holds the part for writing Steward plugins, imported from
`steward_sdk.plugin`. A plugin declares what it needs to connect to the tools it works with and what it
manages there, and implements the calls Steward makes. The SDK handles everything else: the gRPC
services, the handshake with the runner, the health check, and routing each call to your handler by
kind.

```sh
pip install treno-dev-steward-sdk
```

```python
from steward_sdk.plugin import create_plugin
```

Requires Python 3.10 or newer. The package ships its type hints.

## Start a plugin

```sh
steward plugin init my-plugin --language python
cd my-plugin
python -m venv .venv && source .venv/bin/activate
pip install -e .
steward plugin validate python src/plugin.py
```

`steward plugin init` takes its templates from the SDK release, so the project always matches the
SDK it depends on. To work on the SDK and its templates together, point it at a local checkout with
`--sdk-path /path/to/steward-sdk`. The template lives next to the SDKs, in `../templates/python`.

## A plugin

`steward plugin init --language python` generates a plugin with a resource that can be provisioned and
given access to. The whole file, with every call and its comments, is
[`templates/python/src/plugin.py.tmpl`](https://github.com/treno-dev/steward-sdk/blob/main/templates/python/src/plugin.py.tmpl).
In short:

```python
from steward_sdk.plugin import create_plugin


async def provision(request):
    base_url = request.integration.inputs["base_url"]

    return {"outputs": {"url": f"{base_url}/items/{request.name}"}}


async def deprovision(request):
    return {}


plugin = create_plugin(
    name="my-plugin",
    version="0.1.0",
    inputs=[{"name": "base_url", "label": "Base URL", "type": "string", "required": True}],
    secrets=[{"name": "API_TOKEN", "description": "The token to call the API with.", "required": True}],
)

plugin.resource(
    kind="item",
    outputs=[{"name": "url", "label": "URL", "type": "string"}],
    provision=provision,
    deprovision=deprovision,
)

plugin.serve()
```

A plugin needs one `create_plugin(...)`, any number of `plugin.resource(...)` and
`plugin.application(...)` declarations, and a final `plugin.serve()`. If it needs more than one tool,
for example both Cloudflare and AWS, take the credentials for each as secrets.

## What you declare

**The plugin**, in `create_plugin`:

| Argument                                                              |                                                                                                                                                      |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| `name`, `version`                                                     | Required.                                                                                                                                            |
| `title`, `description`                                                | Shown in Steward.                                                                                                                                    |
| `inputs`                                                              | The settings the plugin is configured with.                                                                                                          |
| `secrets`                                                             | The credentials it is configured with, such as an API token.                                                                                         |
| `validate`                                                            | Optional. Checks the credentials, which Steward cannot. Returns `{"errors": [{"field": ..., "message": ...}]}`. Without it, every integration is accepted. |
| `roles`, `permissions`, `grant_access`, `revoke_access`, `get_access` | Access to the plugin as a whole, such as membership of an organization.                                                                              |

**A resource**, with `plugin.resource(kind=..., ...)`, and **an application**, with
`plugin.application(kind=..., ...)`, both have a `kind` that is unique within the plugin, a `title`, a
`description`, `inputs`, `secrets` and `outputs`. Resources also take `roles` and `permissions`. Applications take
`sources`, the types they can be deployed from: `"github"`, `"registry"`, `"s3"` or `"raw"`. The `source`
of an application arrives as `type`, `config` and `ref`, where `config` holds the settings of that type:
`github` `{owner, name, branch?}`, `registry` `{image, tag?}`, `s3` `{bucket, key}` and `raw` `{path}`.
`ref` pins a commit, an image digest or an object version, and is empty for the newest.

**Inputs and outputs** are declared like variables:

```python
{"name": "visibility", "label": "Visibility", "type": "select", "options": ["private", "public"], "default": "private"}
```

`type` is one of `string`, `number`, `boolean`, `select`, `list` (of strings) or `map`. Mark an output
`"sensitive": True` when it is a secret, such as a generated password, and it stays with the runner.

**Secrets** are credentials, and are kept, sent and shown apart from inputs. Each has a `name`, used as
it is, a `description` and whether it is `required`. Its value is never shown back once set.

```python
{"name": "GITHUB_TOKEN", "description": "A token that can create repositories.", "required": True}
```

## Access

Access exists at two levels, each with the same three handlers, `grant_access`, `revoke_access` and
`get_access`:

- **The plugin**, in `create_plugin`: membership of the tool as a whole, such as an organization or a
  site. Steward grants this first, then access to the resources.
- **A resource kind**, in `plugin.resource`: a role on one resource, such as a repository. The
  generated plugin above shows this.

For the plugin it looks like this:

```python
async def grant_access(request: PluginGrantAccessRequest) -> PluginGrantAccessResponse:
    identity = request.identity
    email = identity.external_id or plain(identity.attrs)["email"]
    invitation = await invite(request.integration, email, request.role.name)

    return {"id": invitation.id, "role": request.role, "pending": True}  # an invitation is not access until accepted


plugin = create_plugin(
    name="github",
    version="0.1.0",
    inputs=[...],
    roles=[{"name": "member", "title": "Member"}, {"name": "owner", "title": "Owner"}],
    grant_access=grant_access,
    revoke_access=revoke_access,
    get_access=get_access,
)
```

`invite` stands for a call to the tool's own API.

A resource kind takes the same three handlers and its own `roles` and `permissions`. Its requests also
carry the `resource`:

```python
async def grant_access(request: ResourceGrantAccessRequest) -> ResourceGrantAccessResponse:
    await add_collaborator(request.integration, request.resource.name, request.identity.external_id, request.role.name)

    return {"role": plain(request.role), "permissions": [plain(item) for item in request.permissions]}


plugin.resource(
    kind="repository",
    permissions=[{"name": "pull"}, {"name": "push"}],
    roles=[{"name": "viewer", "title": "Viewer"}, {"name": "editor", "title": "Editor"}],
    grant_access=grant_access,
    revoke_access=revoke_access,
    get_access=get_access,
)
```

Declaring `roles` or `permissions` is what says that the plugin, or the kind, supports access. They are
two separate lists:

- **`roles`** are the ones the tool defines itself, each with a `name`, `title` and `description`.
- **`permissions`** are the pieces Steward builds its own roles from. A permission is a `name` with an
  optional `level`: `{"name": "pull_requests", "level": "read"}`, or just `{"name": "s3:GetObject"}`.

Handlers receive the `identity` (`external_id`, `name`, `attrs`) and the `role` to apply, and a resource
call also gets the `resource` (`kind`, `name`). The `role` is always set. For a role Steward composed
from your permissions, `permissions` lists its pieces too, for you to apply as far as the tool allows.
They return:

- `grant_access`: the `role` and `permissions` as the tool applied them, which may differ from what was
  requested, an `id` for the grant if the tool has one (Steward sends it back on revoke), and
  `pending: True` while the person has to act first, such as accepting an invitation.
- `revoke_access`: nothing. It succeeds if the identity did not have the role.
- `get_access`: the `roles` and `permissions` the identity holds now, and whether a grant is still
  `pending`.

Steward works out overlap between roles before it asks you to revoke one, so you can remove the role
you are given in full.

## What Steward offers

Steward offers what you implement. A call for a handler you did not pass answers `UNIMPLEMENTED`, and
Steward treats that as not offered. Access also needs its `roles` or `permissions`, which tell Steward
what can be granted.

| You pass                                                       | The kind can                                                                                                                 |
| -------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `provision`, `deprovision`                                     | be created and removed                                                                                                       |
| `list`                                                         | be discovered (optional: Steward keeps track of what it creates, so `list` is only for finding resources that already exist) |
| `roles` or `permissions`, with `grant_access`, `revoke_access`, `get_access` | have access granted (resources and the plugin)                                                                 |
| `create`, `destroy`, `deploy`                                  | be run as an application                                                                                                     |

## Handlers

A handler takes the request and returns the response. It may be `async def` or a plain function (a
plain one runs in a thread, so blocking calls do not stall the plugin).

- **Requests** are the generated protobuf messages, so fields are snake_case and your editor completes
  them. A message the runner did not send reads as empty. A Struct such as `inputs` supports `[]`,
  `in` and `dict()`; `plain(message)` turns any message into a plain dict.
- **Responses** are a response message or a plain dict with the fields you have something to say
  about (snake_case or camelCase). Messages inside it, such as the `role` you were given, can be used
  as they are. Returning nothing is an empty response.
- **Inputs and secrets.** `integration.inputs` and `inputs` hold the values of the inputs, keyed by input
  name; numbers arrive as floats. The values of secrets arrive apart from them, keyed by secret name,
  in `integration.secrets` (and a resource's or application's `secrets`), a plain `dict`-like of strings.
  Never store them.
- **Idempotent, and no report of changes.** Make calls that change something idempotent: creating what
  exists, or removing what is gone, simply succeeds. Return the result of the change (the `outputs`,
  the applied `role`), not a description of what changed: Steward keeps the state and works out the
  difference itself.
- **Resources and identities.** A resource is identified by its `kind` and `name`, which Steward stores
  and sends on every later call. `provision` receives them with the `inputs` and returns only the
  `outputs`, such as a URL or an id the tool assigned. An identity has `external_id`, `name`, `ulid`
  and `attrs`; those that are optional report whether they were set with `HasField`.
- **Variables.** An application's variables each carry `sensitive`, `sealed` and `locked`. A `deploy`
  carries the application's full set of variables, so changing them is a deploy. Fail it with
  `FAILED_PRECONDITION` if it would change or remove a locked variable.

## Errors

Raise a `PluginError` with a gRPC status code to report a specific failure:

```python
from steward_sdk.plugin import PluginError, StatusCode

raise PluginError(StatusCode.FAILED_PRECONDITION, "APP_KEY is locked and cannot be removed")
```

Common codes: `INVALID_ARGUMENT`, `NOT_FOUND`, `FAILED_PRECONDITION`, `UNIMPLEMENTED`. Any other
exception is reported as `INTERNAL`, with its traceback on stderr. A call for an unknown kind is
answered with `NOT_FOUND` by the SDK.

## Logging

Write logs to standard error (the `logging` module does by default), which the runner collects. What a
plugin prints to standard output after the handshake is not shown.

## Developing the SDK

The messages and gRPC services are generated from `../proto` with `buf` (`buf.gen.yaml`, using
buf's remote plugins, so it needs network access), and the generated code is not committed. The plugin
versions in `buf.gen.yaml` decide the minimum `protobuf` and `grpcio` the package requires.

```sh
python -m venv .venv && source .venv/bin/activate
pip install -e ".[dev]"
buf generate        # the messages into src/steward_sdk/_gen
python -m build     # the wheel and sdist, with the generated code
```
