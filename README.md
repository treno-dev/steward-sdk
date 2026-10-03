# Steward SDK

Everything you need to write a Steward plugin: the contract, SDKs for Go, JavaScript, TypeScript and
Python, and the templates `steward plugin init` starts a project from.

A plugin connects Steward to one tool, the way a provider does in Terraform. It says what it needs to
connect (a URL, a token), what it can manage there (repositories, buckets, projects) and what can be
granted on those (roles, permissions), and it carries out the calls Steward makes: create this, remove
that, give this person that role. Steward decides what should happen and keeps the state. The plugin
only does it.

## How it works

1. **A plugin is a program.** You write it with an SDK and build or run it like any other. Steward
   never runs plugin code itself: a runner, which people operate in their own environment, starts the
   plugin as a subprocess when it needs it.
2. **The runner and the plugin speak gRPC.** The plugin listens on a local port and prints one line,
   the handshake, so the runner knows where to connect. Then the runner describes the plugin, checks
   its health and calls it.
3. **The contract is a protobuf file.** [`proto/steward/plugin/v1/plugin.proto`](proto/steward/plugin/v1/plugin.proto)
   defines every message and call. The SDKs are generated from it, so they all behave the same way.
   Within `v1` the contract only grows.
4. **The SDK hides the protocol.** You declare the integration and its kinds of resources, and write
   one function per call. The SDK serves gRPC, prints the handshake, answers the health check, routes
   each call to your function by kind, and answers anything you did not write with `UNIMPLEMENTED`.

### The pieces of a plugin

- **An integration**: one connection to the tool. It declares its **inputs**, like variables with a
  name and a type. Credentials are inputs marked `sensitive`, and the runner passes them to each call
  separately so the plugin never has to store them.
- **Resources**: the things the tool holds, each declared once under a **kind** (`"repository"`) with
  its own inputs and **outputs** (what comes back once it exists, such as a URL). A resource is
  identified by its kind and name.
- **Access**: roles and permissions that can be granted on the integration as a whole, or on one
  resource. A plugin declares what the tool offers, and Steward can build custom roles from the
  permissions.
- **Applications** (optional): things that are deployed from a source and have open-ended variables,
  as opposed to resources, which are provisioned.

Capabilities are never listed. The SDK reads them from the functions you wrote: with `provision` and
`deprovision` a kind can be created and removed, with `grantAccess`, `revokeAccess` and `getAccess` it
can have access granted, with `list` it can be discovered.

Calls that change something are **idempotent** and return the **result** (the outputs, the role as the
tool applied it), not a report of what changed. Steward works out the difference itself.

## A plugin, in JavaScript

```sh
steward plugin init my-plugin --language js    # or ts, python, go
cd my-plugin
npm install
npm start
```

This is the heart of what that generates, shortened. The full file, with every call and its comments,
is [`templates/js/src/plugin.js.tmpl`](templates/js/src/plugin.js.tmpl).

```js
import { createPlugin } from '@steward/plugin';

// The integration: what it takes to connect to the tool.
const plugin = createPlugin({
  name: 'my-plugin',
  version: '0.1.0',
  title: 'My plugin',

  inputs: [
    { name: 'base_url', label: 'Base URL', type: 'string', required: true },
    { name: 'token', label: 'API token', type: 'string', required: true, sensitive: true },
  ],

  // Return an error for each input the user can fix.
  async validate({ integration }) {
    const errors = [];

    if (!integration.secrets.token) {
      errors.push({ field: 'token', message: 'An API token is required.' });
    }

    return { errors };
  },
});

// A kind of resource it manages.
plugin.resource({
  kind: 'item',
  title: 'Item',
  outputs: [{ name: 'url', label: 'URL', type: 'string' }],
  roles: [{ name: 'read', title: 'Read' }, { name: 'write', title: 'Write' }],

  // Create the resource. Idempotent. Return what it produced.
  async provision({ integration, name }) {
    return { outputs: { url: `${integration.inputs.base_url}/items/${name}` } };
  },

  async deprovision({ resource }) {
    return {};
  },

  // Give an identity a role on the resource. Return the role as the tool applied it.
  async grantAccess({ identity, role }) {
    return { role };
  },

  async revokeAccess({ identity, role }) {
    return {};
  },

  async getAccess({ identity }) {
    return { roles: [], pending: false };
  },
});

plugin.serve();
```

Run it by hand and it prints the handshake line and waits:

```
1|1|tcp|127.0.0.1:54010|grpc
```

Everything after that is the runner calling it. Handlers receive plain objects and may leave out any
field of the response they have nothing to say about. To fail a call, throw `{ code, message }` with a
gRPC status code. Logs go to stderr, because stdout is reserved for the handshake.

The other languages follow the same shape, in their own idiom:

| Language | Declare | A handler |
|---|---|---|
| JavaScript, TypeScript | `createPlugin({...})`, `plugin.resource({ kind, ... })` | `async provision({ integration, name }) { ... }` |
| Python | `create_plugin(...)`, `plugin.resource(kind=..., ...)` | `async def provision(request): ...` |
| Go | `steward.New(...)`, `plugin.Resource("kind", ...)` | `func(ctx, *Request) (*Response, error)` |

## What is here

| | |
|---|---|
| [`proto/`](proto) | The contract, with `buf` lint and breaking-change checks. |
| [`js/`](js) | `@steward/plugin` for JavaScript and TypeScript. |
| [`python/`](python) | `steward-plugin` for Python. |
| [`go/`](go) | The Go module, `github.com/treno-dev/steward-sdk/go`. |
| [`templates/`](templates) | One template per language (`js`, `ts`, `python`, `go`), the projects `steward plugin init` writes. |
| [`pack-templates.sh`](pack-templates.sh) | Packs the templates into the archive a release publishes. |

Each SDK has a README with everything a plugin can declare: [JavaScript](js/README.md),
[Python](python/README.md), [Go](go/README.md).

## Templates

`steward plugin init` does not carry its examples. It downloads the templates of an SDK release and
checks them against their checksum, so a new project always matches the SDK it depends on. To work on
an SDK and its template together, point the CLI at a checkout of this repository:

```sh
steward plugin init my-plugin --language js --sdk-path /path/to/steward-sdk
```

## Developing

The SDKs are generated from the contract with `buf` (remote plugins, so it needs network access).
The JavaScript and Python builds generate on demand and do not commit the result. The Go module
commits `go/gen`, because Go users install a module as it is.

```sh
cd proto && buf lint && buf build && buf format -d    # check the contract
cd js && npm install && npm run build                  # JavaScript and TypeScript
cd python && python scripts/generate.py                # Python
cd go && buf generate && go vet ./...                  # Go
```

Changing the contract: add, never renumber or remove (`buf breaking` fails any change that does).
Regenerate every SDK, and keep each template in step, because the READMEs embed their examples.
