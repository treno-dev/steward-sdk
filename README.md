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

| System | Resources it provisions | Applications it runs | Access it grants |
|---|---|---|---|
| GitHub | repositories, teams | | `read`, `write`, `admin` on a repository |
| AWS | buckets, databases, accounts | containers, functions | policies made of actions such as `s3:GetObject` |
| A hosting platform | sites, databases | web apps deployed from git | team membership |
| Jira | projects | | project roles such as `Developer` |

Steward needs to do the same few things in all of them: create things, remove things, deploy things,
let someone in, change what they can do, take it away. But "create a bucket" or "give Amina write
access" means a completely different API call in each system.

## What a plugin is

A **plugin** is a small program that teaches Steward one external system. It is the translator between
Steward's questions and that system's API. The GitHub plugin, for example:

- **declares what it needs to connect:** the GitHub organization and an access token. Steward shows
  these as a form when someone connects GitHub, and calls each connection an **integration**. A
  company can have several integrations from one plugin, such as two GitHub organizations.
- **declares what it manages:** kinds of **resources**, such as `repository`, each with the settings it
  takes to create one (visibility, description) and what it reports back once it exists (its URL).
- **declares what can be granted:** the roles and permissions GitHub offers, such as `read`, `write`
  and `admin` on a repository.
- **carries out Steward's calls:** create the repository `payments-api`, give `amina` the role `write`
  on it, take it away again, tell me what `amina` holds right now.

Those calls fall into three kinds of work. A plugin supports whichever fit its system:

- **Resource provisioning:** create and remove resources, and report what they produced, such as a URL
  or an id. Optionally, list resources that already exist so they can be brought under Steward.
- **Application management:** create, deploy and delete applications, which are run from a source
  (a git repository or an image) and have variables, such as a web app and its environment.
- **Access:** grant, revoke and report someone's roles, on the system as a whole (membership of the
  GitHub organization) or on one resource (a role on one repository).

The plugin declares which of these it supports, for the system as a whole and for each kind of
resource, and Steward offers only that. The list is designed to grow: new kinds of work are added to
the contract beside these without breaking plugins that already exist.

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
2. **The runner and the plugin speak gRPC.** The plugin listens on a local port and prints one line,
   the handshake, so the runner knows where to connect. Then the runner describes the plugin, checks
   its health and calls it.
3. **The contract is a protobuf file.** [`proto/steward/plugin/v1/plugin.proto`](proto/steward/plugin/v1/plugin.proto)
   defines every message and call. The SDKs are generated from it, so they all behave the same way.
   Within `v1` the contract only grows.
4. **The SDK hides the protocol.** You declare the integration and its kinds of resources, and write
   one function per call. The SDK serves gRPC, prints the handshake, answers the health check, routes
   each call to your function by kind, and answers anything you did not write with `UNIMPLEMENTED`.

### Any language works

The SDKs are a convenience, not a requirement. The runner speaks the protocol of
[`hashicorp/go-plugin`](https://github.com/hashicorp/go-plugin) (a subprocess, a handshake line, gRPC),
so a plugin can be written in any language that can serve gRPC. A program is a Steward plugin if it:

1. **prints the handshake line** on stdout, `1|1|tcp|127.0.0.1:<port>|grpc`, once it is listening;
2. **serves the services of the contract**: `PluginService` always, and `IntegrationService`,
   `ResourceService` and `ApplicationService` for what it supports, generated from
   [the proto](proto/steward/plugin/v1/plugin.proto) with `protoc` or `buf` in your language; and
3. **serves the standard gRPC health service**, reporting `SERVING` for the service named `plugin`.

That is the whole obligation. Nothing else is needed, and the plugin never has to import Steward code.

What the SDK adds is what makes writing one easy: it does those three things for you, turns the
`google.protobuf.Struct` values into plain objects, routes each call to your function by kind, builds
the description from what you declared, fills in what a handler leaves out, and answers what you did
not write with `UNIMPLEMENTED`. So the SDKs for JavaScript, TypeScript, Python and Go are the easy
road, and a plugin in Rust, Java or anything else is the same contract without that help.

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
