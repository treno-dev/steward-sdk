# @treno-dev/steward-sdk

The Steward SDK for JavaScript and TypeScript. Today it holds the part for writing Steward plugins,
imported from `@treno-dev/steward-sdk/plugin`. A plugin declares what it needs to connect to the tools
it works with and what it manages there, and implements the calls Steward makes. The SDK handles
everything else: the gRPC services, the handshake with the runner, the health check, and routing each
call to your handler by kind.

```sh
npm install @treno-dev/steward-sdk
```

```js
import { createPlugin } from '@treno-dev/steward-sdk/plugin';
```

Requires Node.js (tested on 24). The package is ES modules and ships its TypeScript types.

## Start a plugin

```sh
steward plugin init my-plugin --language js    # or --language ts
cd my-plugin
npm install
steward plugin validate node src/plugin.js
```

`steward plugin init` takes its templates from the SDK release, so the project always matches the
SDK it depends on. To work on the SDK and its templates together, point it at a local checkout with
`--sdk-path /path/to/steward-sdk`. The templates live next to the SDKs, in `../templates/js` and
`../templates/ts`.

## A plugin

`steward plugin init --language js` generates a plugin with a resource that can be provisioned and given
access to. The whole file, with every call and its comments, is
[`templates/js/src/plugin.js.tmpl`](https://github.com/treno-dev/steward-sdk/blob/main/templates/js/src/plugin.js.tmpl).
In short:

```js
import { createPlugin } from '@treno-dev/steward-sdk/plugin';

const plugin = createPlugin({
  name: 'my-plugin',
  version: '0.1.0',
  inputs: [
    { name: 'base_url', label: 'Base URL', type: 'string', required: true },
    { name: 'token', label: 'API token', type: 'string', required: true, sensitive: true },
  ],
});

plugin.resource({
  kind: 'item',
  outputs: [{ name: 'url', label: 'URL', type: 'string' }],

  async provision({ config, name }) {
    return { outputs: { url: `${config.inputs.base_url}/items/${name}` } };
  },

  async deprovision({ resource }) {
    return {};
  },
});

plugin.serve();
```

A plugin needs one `createPlugin(...)`, any number of `plugin.resource(...)` and
`plugin.application(...)` declarations, and a final `plugin.serve()`. If it needs more than one tool,
for example both Cloudflare and AWS, take the credentials for each as inputs.

## What you declare

**The plugin**, in `createPlugin`:

| Option | |
|---|---|
| `name`, `version` | Required. |
| `title`, `description` | Shown in Steward. |
| `inputs` | What is required to configure the plugin, credentials included. |
| `validate` | Optional. Checks the credentials, which Steward cannot. Returns `{ errors: [{ field, message }] }`. Without it, every config is accepted. |
| `roles`, `permissions`, `grantAccess`, `revokeAccess`, `getAccess` | Access to the plugin as a whole, such as membership of an organization. |

**A resource**, with `plugin.resource({ kind, ... })`, and **an application**, with
`plugin.application({ kind, ... })`, both have a `kind` that is unique within the plugin, a `title`,
a `description`, `inputs` and `outputs`. Resources also take `roles` and `permissions`. Applications
take `sources`, the types they can be deployed from: `'github'`, `'registry'`, `'s3'` or `'raw'`. The
`source` of an application arrives as `{ type, config, ref }`, where `config` holds the settings of that
type: `github` `{ owner, name, branch? }`, `registry` `{ image, tag? }`, `s3` `{ bucket, key }` and `raw`
`{ path }`. `ref` pins a commit, an image digest or an object version, and is empty for the newest.

**Inputs and outputs** are declared like variables:

```js
{ name: 'visibility', label: 'Visibility', type: 'select', options: ['private', 'public'], default: 'private' }
```

`type` is one of `string`, `number`, `boolean`, `select`, `list` (of strings) or `map`. Mark a
credential `sensitive: true` and it is never shown back once set.

## Access

Access exists at two levels, each with the same three handlers, `grantAccess`, `revokeAccess` and
`getAccess`:

- **The plugin**, in `createPlugin`: membership of the tool as a whole, such as an organization or a
  site. Steward grants this first, then access to the resources.
- **A resource kind**, in `plugin.resource`: a role on one resource, such as a repository. The
  generated plugin above shows this.

For the plugin it looks like this:

```js
const plugin = createPlugin({
  name: 'github',
  version: '0.1.0',
  inputs: [/* ... */],

  roles: [{ name: 'member', title: 'Member' }, { name: 'owner', title: 'Owner' }],

  grantAccess: async ({ config, identity, role }) => {
    const invitation = await invite(config, identity.externalId || identity.attrs.email, role.name);

    return { id: invitation.id, role, pending: true }; // an invitation is not access until accepted
  },
  revokeAccess: async ({ config, identity }) => {
    await removeFromOrganization(config, identity.externalId);

    return {};
  },
  getAccess: async ({ config, identity }) => {
    const membership = await findMembership(config, identity);

    return { roles: membership ? [{ name: membership.role }] : [], pending: Boolean(membership?.pending) };
  },
});
```

`invite`, `removeFromOrganization` and `findMembership` stand for calls to the tool's own API.

Declaring `roles` or `permissions` is what says that the plugin, or the kind, supports access. They are
two separate lists:

- **`roles`** are the ones the tool defines itself, each with a `name`, `title` and `description`.
- **`permissions`** are the pieces Steward builds its own roles from. A permission is a `name` with an
  optional `level`: `{ name: 'pull_requests', level: 'read' }`, or just `{ name: 's3:GetObject' }`.

Handlers receive the `identity` (`externalId`, `name`, `attrs`) and the `role` to apply, and a resource
call also gets the `resource` (`kind`, `name`). The `role` is always set. For a role Steward composed
from your permissions, `permissions` lists its pieces too, for you to apply as far as the tool allows.
They return:

- `grantAccess`: the `role` and `permissions` as the tool applied them, which may differ from what was
  requested, an `id` for the grant if the tool has one (Steward sends it back on revoke), and
  `pending: true` while the person has to act first, such as accepting an invitation.
- `revokeAccess`: nothing. It succeeds if the identity did not have the role.
- `getAccess`: the `roles` and `permissions` the identity holds now, and whether a grant is still
  `pending`.

Steward works out overlap between roles before it asks you to revoke one, so you can remove the role
you are given in full.

## What you write is what is offered

You never list what a kind can do. Steward calls, and a call for something you did not write fails
with `UNIMPLEMENTED`, which means it is not offered. The one thing you declare is access: roles or
permissions say that it is supported.

| You write | The kind can |
|---|---|
| `provision`, `deprovision` | be created and removed |
| `list` | be discovered (optional: Steward keeps track of what it creates, so `list` is only for finding resources that already exist) |
| `roles` or `permissions`, with `grantAccess`, `revokeAccess`, `getAccess` | have access granted (resources and the plugin) |
| `create`, `delete`, `deploy`, `setVariables`, `list` | be run as an application |

## Handlers

A handler receives the request as a plain object and returns the response as one. It may leave out
any field it has nothing to say about. Field names are camelCase. In TypeScript, requests and
responses are typed from the contract.

- **Inputs.** `config.inputs` and `inputs` hold the values that are not sensitive, keyed by
  input name. Sensitive values arrive in `config.secrets` (and a resource's or application's
  `secrets`). Never store them. The SDK always provides these objects, empty if nothing was set.
- **Idempotent, and no report of changes.** Make calls that change something idempotent: creating
  what exists, or removing what is gone, simply succeeds. Return the result of the change (the
  `outputs`, the applied `role`), not a description of what changed: Steward keeps the state and
  works out the difference itself.
- **Resources and identities.** A resource is identified by its `kind` and `name`, which Steward
  stores and sends on every later call. `provision` receives them with the `inputs` and returns only
  the `outputs`, such as a URL or an id the tool assigned. An identity has
  `externalId`, `name`, `ulid` and `attrs`.
- **Access.** A grant returns the `role` and `permissions` as the tool applied them, an `id` for the
  grant if the tool has one, and `pending: true` while it needs the person to act, such as accepting
  an invitation.
- **Variables.** An application's variables each carry `sensitive`, `sealed` and `locked`. Refuse to
  change or remove a locked variable.

## Errors

Throw an object with a gRPC status code to report a specific failure:

```js
throw { code: 9, message: 'APP_KEY is locked and cannot be removed' }; // FAILED_PRECONDITION
```

Common codes: `3` invalid argument, `5` not found, `9` failed precondition, `12` unimplemented.
Anything else you throw is reported as an internal error. A call for an unknown kind is answered with
`NOT_FOUND` by the SDK.

## Logging

Write logs to standard error (`console.error`), which the runner collects. What a plugin prints to
standard output after the handshake is not shown.

## Developing the SDK

The messages and gRPC services are generated from `../proto` with `buf`, and the generated code is
not committed.

```sh
npm install
buf generate         # the messages into src/gen
npm run build        # tsc into dist/
```
