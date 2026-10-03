# steward-plugin

The Python SDK for writing Steward plugins. A plugin is one integration, like a provider: it declares
what it needs to connect to a tool and what it manages there, and implements the calls Steward makes.
The SDK handles everything else: the gRPC services, the handshake with the runner, the health check,
and routing each call to your handler by kind.

Requires Python 3.10 or newer. The package ships its type hints.

## Start a plugin

```sh
steward plugin init my-plugin --language python
cd my-plugin
python -m venv .venv && source .venv/bin/activate
pip install -e .
python src/plugin.py
```

`steward plugin init` takes its templates from the SDK release, so the project always matches the
SDK it depends on. To work on the SDK and its templates together, point it at a local checkout with
`--sdk-path /path/to/steward-sdk`. The template lives next to the SDKs, in `../templates/python`.

## A plugin

This is the plugin that `steward plugin init --language python` generates, with a resource that can be
provisioned and given access to. The block below is filled in from the template by
`python scripts/readme.py`, so the two never differ.

<!-- template: ../templates/python/src/plugin.py.tmpl -->
```python
# This is the file you edit. It declares what your plugin offers and implements the calls Steward
# makes. The gRPC plumbing, the handshake and the health check live in steward_plugin.
#
# A plugin is one integration, like a provider. Declare it and each kind of resource once, with
# its handlers attached: Steward's forms and the capabilities of each kind (provisioning,
# discovery, access) follow from what you write here, and every call is routed to the right
# handler by kind. If your plugin needs more than one tool, take the credentials for each as inputs.
#
# Each handler is annotated with the request and response types the SDK exports, named for the
# entity and the call (ResourceProvisionRequest, ResourceProvisionResponse). A request is the
# generated message, so your editor completes its fields; a response is a message or a plain dict
# with the fields you have something to say about. Handlers may be async or plain functions.

from steward_plugin import (
    IntegrationValidateRequest,
    IntegrationValidateResponse,
    ResourceDeprovisionRequest,
    ResourceDeprovisionResponse,
    ResourceGetAccessRequest,
    ResourceGetAccessResponse,
    ResourceGrantAccessRequest,
    ResourceGrantAccessResponse,
    ResourceProvisionRequest,
    ResourceProvisionResponse,
    ResourceRevokeAccessRequest,
    ResourceRevokeAccessResponse,
    create_plugin,
)


# Check that the inputs and credentials work. Return an error per input the user can fix.
async def validate(request: IntegrationValidateRequest) -> IntegrationValidateResponse:
    errors = []

    if not request.integration.secrets.get("token"):
        errors.append({"field": "token", "message": "An API token is required."})

    return {"errors": errors}


# Create a resource called `request.name`. Idempotent: if it already exists, succeed anyway. Return
# what it produced, as declared in `outputs`. The request also has `integration` and `inputs`.
async def provision(request: ResourceProvisionRequest) -> ResourceProvisionResponse:
    base_url = request.integration.inputs["base_url"]

    return {"outputs": {"url": f"{base_url}/items/{request.name}"}}


# Remove a resource. Idempotent: succeed if it is already gone. The request has `integration` and
# `resource` (`kind`, `name`).
async def deprovision(request: ResourceDeprovisionRequest) -> ResourceDeprovisionResponse:
    return {}


# Give an identity a role on the resource. Idempotent. Return the role as the tool applied it, an
# `id` for the grant if the tool has one, and `pending: True` if the person has to act first, such
# as accepting an invitation. The request has `integration`, `resource`, `identity` (`external_id`,
# `name`, `attrs`) and `role`.
async def grant_access(request: ResourceGrantAccessRequest) -> ResourceGrantAccessResponse:
    return {"role": request.role}


# Take a role away. Idempotent: succeed if the identity does not have it.
async def revoke_access(request: ResourceRevokeAccessRequest) -> ResourceRevokeAccessResponse:
    return {}


# Report the roles the identity holds on the resource now, and whether a grant is still pending.
async def get_access(request: ResourceGetAccessRequest) -> ResourceGetAccessResponse:
    return {"roles": [], "pending": False}


# The integration is one connection to the tool. Steward builds its form from `inputs`, so it is
# also the documentation people see. Mark credentials `sensitive`: they arrive in
# `request.integration.secrets`, the other inputs in `request.integration.inputs`.
plugin = create_plugin(
    name="my-plugin",
    version="0.1.0",
    title="My plugin",
    description="Connects Steward to My plugin.",
    # What is required to create an integration from this plugin.
    inputs=[
        {"name": "base_url", "label": "Base URL", "type": "string", "required": True},
        {"name": "token", "label": "API token", "type": "string", "required": True, "sensitive": True},
    ],
    validate=validate,
)

# A kind of resource this integration manages.
plugin.resource(
    kind="item",
    title="Item",
    description="An example resource.",
    # What a resource of this kind takes besides its name, which Steward stores with the kind.
    inputs=[{"name": "description", "label": "Description", "type": "string"}],
    outputs=[{"name": "url", "label": "URL", "type": "string"}],
    # What can be granted on this kind: the tool's roles, and the permissions each contains. Writing
    # the three access handlers is what makes the kind accept access.
    roles=[
        {"name": "read", "title": "Read", "permissions": [{"name": "view"}]},
        {"name": "write", "title": "Write", "permissions": [{"name": "view"}, {"name": "edit"}]},
    ],
    provision=provision,
    deprovision=deprovision,
    grant_access=grant_access,
    revoke_access=revoke_access,
    get_access=get_access,
)

plugin.serve()
```
<!-- /template -->

A plugin needs one `create_plugin(...)`, any number of `plugin.resource(...)` and
`plugin.application(...)` declarations, and a final `plugin.serve()`. If it needs more than one tool,
for example both Cloudflare and AWS, take the credentials for each as inputs.

## What you declare

**The integration**, in `create_plugin`:

| Argument | |
|---|---|
| `name`, `version` | Required. |
| `title`, `description` | Shown in Steward. |
| `inputs` | What is required to create an integration, credentials included. |
| `validate` | Checks the inputs and credentials. Returns `{"errors": [{"field": ..., "message": ...}]}`. |
| `roles`, `permissions`, `grant_access`, `revoke_access`, `get_access` | Access to the integration as a whole, such as membership of an organization. |

**A resource**, with `plugin.resource(kind=..., ...)`, and **an application**, with
`plugin.application(kind=..., ...)`, both have a `kind` that is unique within the plugin, a `title`, a
`description`, `inputs` and `outputs`. Resources also take `roles` and `permissions`. Applications take
`sources` (`"git"`, `"image"`).

**Inputs and outputs** are declared like variables:

```python
{"name": "visibility", "label": "Visibility", "type": "select", "options": ["private", "public"], "default": "private"}
```

`type` is one of `string`, `number`, `boolean`, `select`, `list` (of strings) or `map`. Mark a
credential `"sensitive": True` and it is never shown back once set.

## Access

Access exists at two levels, each with the same three handlers, `grant_access`, `revoke_access` and
`get_access`:

- **The integration**, in `create_plugin`: membership of the tool as a whole, such as an organization
  or a site. Steward grants this first, then access to the resources.
- **A resource kind**, in `plugin.resource`: a role on one resource, such as a repository. The
  generated plugin above shows this.

For the integration it looks like this:

```python
async def grant_access(request: IntegrationGrantAccessRequest) -> IntegrationGrantAccessResponse:
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

What you declare with `roles` (and, for tools that expose them, `permissions`) is what Steward offers
for assignment. A role has a `name` and the `permissions` it contains, and a permission is a `name`
with an optional `level`: `{"name": "pull_requests", "level": "read"}`, or just `{"name": "s3:GetObject"}`.
Steward can also build its own roles from the permissions you declare.

Handlers receive the `identity` (`external_id`, `name`, `attrs`) and the `role` to apply, and a resource
call also gets the `resource` (`kind`, `name`). They return:

- `grant_access`: the `role` as the tool applied it, which may differ from the one requested, an `id`
  for the grant if the tool has one (Steward sends it back on revoke), and `pending: True` while the
  person has to act first, such as accepting an invitation.
- `revoke_access`: nothing. It succeeds if the identity did not have the role.
- `get_access`: the `roles` the identity holds now, and whether a grant is still `pending`.

Steward works out overlap between roles before it asks you to revoke one, so you can remove the role
you are given in full.

## Capabilities follow from your handlers

You never list capabilities. The SDK reads them from the handlers you pass:

| You pass | The kind can |
|---|---|
| `provision`, `deprovision` | be created and removed (provisioning) |
| `list` | be discovered (optional: Steward keeps track of what it creates, so `list` is only for finding resources that already exist) |
| `grant_access`, `revoke_access`, `get_access` | have access granted (resources and the integration) |
| `create`, `delete`, `deploy`, `set_variables`, `list` | be run as an application |

A call for something you did not pass fails with `UNIMPLEMENTED`.

## Handlers

A handler takes the request and returns the response. It may be `async def` or a plain function (a
plain one runs in a thread, so blocking calls do not stall the plugin).

- **Requests** are the generated protobuf messages, so fields are snake_case and your editor completes
  them. A message the runner did not send reads as empty. A Struct such as `inputs` supports `[]`,
  `in` and `dict()`; `plain(message)` turns any message into a plain dict.
- **Responses** are a response message or a plain dict with the fields you have something to say
  about (snake_case or camelCase). Messages inside it, such as the `role` you were given, can be used
  as they are. Returning nothing is an empty response.
- **Inputs.** `integration.inputs` and `inputs` hold the values that are not sensitive, keyed by input
  name; numbers arrive as floats. Sensitive values arrive in `integration.secrets` (and a resource's or
  application's `secrets`), a plain `dict`-like of strings. Never store them.
- **Idempotent, and no report of changes.** Make calls that change something idempotent: creating what
  exists, or removing what is gone, simply succeeds. Return the result of the change (the `outputs`,
  the applied `role`), not a description of what changed: Steward keeps the state and works out the
  difference itself.
- **Resources and identities.** A resource is identified by its `kind` and `name`, which Steward stores
  and sends on every later call. `provision` receives them with the `inputs` and returns only the
  `outputs`, such as a URL or an id the tool assigned. An identity has `external_id`, `name`, `ulid`
  and `attrs`; those that are optional report whether they were set with `HasField`.
- **Variables.** An application's variables each carry `sensitive`, `sealed` and `locked`. Refuse to
  change or remove a locked variable.

## Errors

Raise a `PluginError` with a gRPC status code to report a specific failure:

```python
from steward_plugin import PluginError, StatusCode

raise PluginError(StatusCode.FAILED_PRECONDITION, "APP_KEY is locked and cannot be removed")
```

Common codes: `INVALID_ARGUMENT`, `NOT_FOUND`, `FAILED_PRECONDITION`, `UNIMPLEMENTED`. Any other
exception is reported as `INTERNAL`, with its traceback on stderr. A call for an unknown kind is
answered with `NOT_FOUND` by the SDK.

## Logging

Write logs to stderr (the `logging` module does by default). Standard output is reserved for the one
handshake line the runner reads.

## Developing the SDK

The messages and gRPC services are generated from `../proto` with `buf` (`buf.gen.yaml`, using
buf's remote plugins, so it needs network access), and the generated code is not committed. The plugin
versions in `buf.gen.yaml` decide the minimum `protobuf` and `grpcio` the package requires.

```sh
python -m venv .venv && source .venv/bin/activate
pip install -e ".[dev]"
python scripts/generate.py          # the contract into src/steward_plugin/_gen
python scripts/readme.py            # refill the example above from ../templates/python
python scripts/readme.py --check    # fail if the example is out of date (for CI)
python -m build                     # the wheel and sdist, with the generated code
```
