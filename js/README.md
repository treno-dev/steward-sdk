# @treno-dev/steward-sdk

The Steward SDK for JavaScript and TypeScript. Today it holds the part for writing Steward plugins,
imported from `@treno-dev/steward-sdk/plugin`. A plugin is one integration: it declares what it needs
to connect to a system and what it manages there, and implements the calls Steward makes. The SDK
handles everything else: the gRPC services, the handshake with the runner, the health check, and
routing each call to your handler by kind.

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
npm start
```

`steward plugin init` takes its templates from the SDK release, so the project always matches the
SDK it depends on. To work on the SDK and its templates together, point it at a local checkout with
`--sdk-path /path/to/steward-sdk`. The templates live next to the SDKs, in `../templates/js` and
`../templates/ts`.

## A plugin

This is the plugin that `steward plugin init --language js` generates, with a resource that can be
provisioned and given access to. The block below is filled in from the template by `npm run readme`,
so the two never differ.

<!-- template: ../templates/js/src/plugin.js.tmpl -->
```js
// This is the file you edit. It declares what your plugin offers and implements the calls Steward
// makes. The gRPC plumbing, the handshake and the health check live in the Steward SDK.
//
// A plugin is one integration, like a provider. Declare it and each kind of resource once, with
// its handlers attached: Steward's forms and the capabilities of each kind (provisioning,
// discovery, access) follow from what you write here, and every call is routed to the right
// handler by kind. If your plugin needs more than one tool, take the credentials for each as inputs.

import { createPlugin } from '@treno-dev/steward-sdk/plugin';

// The integration is one connection to the tool. Steward builds its form from `inputs`, so it is
// also the documentation people see. Mark credentials `sensitive`: they arrive in
// `integration.secrets`, the other inputs in `integration.inputs`.
const plugin = createPlugin({
  name: 'my-plugin',
  version: '0.1.0',
  title: 'My plugin',
  description: 'Connects Steward to My plugin.',

  // What is required to create an integration from this plugin.
  inputs: [
    { name: 'base_url', label: 'Base URL', type: 'string', required: true },
    { name: 'token', label: 'API token', type: 'string', required: true, sensitive: true },
  ],

  // Check that the inputs and credentials work. Return an error per input the user can fix.
  async validate({ integration }) {
    const errors = [];

    if (!integration.secrets.token) {
      errors.push({ field: 'token', message: 'An API token is required.' });
    }

    return { errors };
  },
});

// A kind of resource this integration manages. Handlers receive the request as a plain object and
// return the response as one, leaving out any field they have nothing to say about.
plugin.resource({
  kind: 'item',
  title: 'Item',
  description: 'An example resource.',
  // What a resource of this kind takes besides its name, which Steward stores with the kind.
  inputs: [{ name: 'description', label: 'Description', type: 'string' }],
  outputs: [{ name: 'url', label: 'URL', type: 'string' }],

  // Create a resource called `name`. Idempotent: if it already exists, succeed anyway. Return what
  // it produced, as declared in `outputs`.
  async provision({ integration, name, inputs }) {
    const url = `${integration.inputs.base_url}/items/${name}`;

    return { outputs: { url } };
  },

  // Remove a resource. Idempotent: succeed if it is already gone.
  async deprovision({ resource }) {
    return {};
  },

  // What can be granted on this kind: the tool's roles, and the permissions each contains. Writing
  // the three access handlers below is what makes the kind accept access.
  roles: [
    { name: 'read', title: 'Read', permissions: [{ name: 'view' }] },
    { name: 'write', title: 'Write', permissions: [{ name: 'view' }, { name: 'edit' }] },
  ],

  // Give an identity a role on the resource. Idempotent. Return the role as the tool applied it,
  // an `id` for the grant if the tool has one, and `pending: true` if the person has to act first,
  // such as accepting an invitation. `identity` has `externalId`, `name` and `attrs` (an email, say).
  async grantAccess({ integration, resource, identity, role }) {
    return { role };
  },

  // Take a role away. Idempotent: succeed if the identity does not have it.
  async revokeAccess({ integration, resource, identity, role }) {
    return {};
  },

  // Report the roles the identity holds on the resource now, and whether a grant is still pending.
  async getAccess({ integration, resource, identity }) {
    return { roles: [], pending: false };
  },
});

plugin.serve();
```
<!-- /template -->

A plugin needs one `createPlugin(...)`, any number of `plugin.resource(...)` and
`plugin.application(...)` declarations, and a final `plugin.serve()`. If it needs more than one tool,
for example both Cloudflare and AWS, take the credentials for each as inputs.

## What you declare

**The integration**, in `createPlugin`:

| Option | |
|---|---|
| `name`, `version` | Required. |
| `title`, `description` | Shown in Steward. |
| `inputs` | What is required to create an integration, credentials included. |
| `validate` | Checks the inputs and credentials. Returns `{ errors: [{ field, message }] }`. |
| `roles`, `permissions`, `grantAccess`, `revokeAccess`, `getAccess` | Access to the integration as a whole, such as membership of an organization. |

**A resource**, with `plugin.resource({ kind, ... })`, and **an application**, with
`plugin.application({ kind, ... })`, both have a `kind` that is unique within the plugin, a `title`,
a `description`, `inputs` and `outputs`. Resources also take `roles` and `permissions`. Applications
take `sources` (`'git'`, `'image'`).

**Inputs and outputs** are declared like variables:

```js
{ name: 'visibility', label: 'Visibility', type: 'select', options: ['private', 'public'], default: 'private' }
```

`type` is one of `string`, `number`, `boolean`, `select`, `list` (of strings) or `map`. Mark a
credential `sensitive: true` and it is never shown back once set.

## Access

Access exists at two levels, each with the same three handlers, `grantAccess`, `revokeAccess` and
`getAccess`:

- **The integration**, in `createPlugin`: membership of the tool as a whole, such as an organization
  or a site. Steward grants this first, then access to the resources.
- **A resource kind**, in `plugin.resource`: a role on one resource, such as a repository. The
  generated plugin above shows this.

For the integration it looks like this:

```js
const plugin = createPlugin({
  name: 'github',
  version: '0.1.0',
  inputs: [/* ... */],

  roles: [{ name: 'member', title: 'Member' }, { name: 'owner', title: 'Owner' }],

  grantAccess: async ({ integration, identity, role }) => {
    const invitation = await invite(integration, identity.externalId || identity.attrs.email, role.name);

    return { id: invitation.id, role, pending: true }; // an invitation is not access until accepted
  },
  revokeAccess: async ({ integration, identity }) => {
    await removeFromOrganization(integration, identity.externalId);

    return {};
  },
  getAccess: async ({ integration, identity }) => {
    const membership = await findMembership(integration, identity);

    return { roles: membership ? [{ name: membership.role }] : [], pending: Boolean(membership?.pending) };
  },
});
```

`invite`, `removeFromOrganization` and `findMembership` stand for calls to the tool's own API.

What you declare with `roles` (and, for tools that expose them, `permissions`) is what Steward offers
for assignment. A role has a `name` and the `permissions` it contains, and a permission is a `name`
with an optional `level`: `{ name: 'pull_requests', level: 'read' }`, or just `{ name: 's3:GetObject' }`.
Steward can also build its own roles from the permissions you declare.

Handlers receive the `identity` (`externalId`, `name`, `attrs`) and the `role` to apply, and a resource
call also gets the `resource` (`kind`, `name`). They return:

- `grantAccess`: the `role` as the tool applied it, which may differ from the one requested, an `id`
  for the grant if the tool has one (Steward sends it back on revoke), and `pending: true` while the
  person has to act first, such as accepting an invitation.
- `revokeAccess`: nothing. It succeeds if the identity did not have the role.
- `getAccess`: the `roles` the identity holds now, and whether a grant is still `pending`.

Steward works out overlap between roles before it asks you to revoke one, so you can remove the role
you are given in full.

## Capabilities follow from your handlers

You never list capabilities. The SDK reads them from the handlers you write:

| You write | The kind can |
|---|---|
| `provision`, `deprovision` | be created and removed (provisioning) |
| `list` | be discovered (optional: Steward keeps track of what it creates, so `list` is only for finding resources that already exist) |
| `grantAccess`, `revokeAccess`, `getAccess` | have access granted (resources and the integration) |
| `create`, `delete`, `deploy`, `setVariables`, `list` | be run as an application |

A call for something you did not write fails with `UNIMPLEMENTED`.

## Handlers

A handler receives the request as a plain object and returns the response as one. It may leave out
any field it has nothing to say about. Field names are camelCase. In TypeScript, requests and
responses are typed from the contract.

- **Inputs.** `integration.inputs` and `inputs` hold the values that are not sensitive, keyed by
  input name. Sensitive values arrive in `integration.secrets` (and a resource's or application's
  `secrets`). Never store them. The SDK always provides these objects, empty if nothing was set.
- **Idempotent, and no report of changes.** Make calls that change something idempotent: creating
  what exists, or removing what is gone, simply succeeds. Return the result of the change (the
  `outputs`, the applied `role`), not a description of what changed: Steward keeps the state and
  works out the difference itself.
- **Resources and identities.** A resource is identified by its `kind` and `name`, which Steward
  stores and sends on every later call. `provision` receives them with the `inputs` and returns only
  the `outputs`, such as a URL or an id the tool assigned. An identity has
  `externalId`, `name`, `ulid` and `attrs`.
- **Access.** A grant returns the `role` as the tool applied it, an `id` for the grant if the tool
  has one, and `pending: true` while it needs the person to act, such as accepting an invitation.
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

Write logs to stderr. Standard output is reserved for the one handshake line the runner reads.

## Developing the SDK

The messages and gRPC services are generated from `../proto` with `buf`, and the generated code is
not committed.

```sh
npm install
npm run build        # buf generate, then tsc into dist/
npm run readme       # refill the example above from ../templates/js
npm run readme:check # fail if the example is out of date (for CI)
```
