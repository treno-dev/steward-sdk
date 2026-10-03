// The plugin part of the Steward SDK for JavaScript: the plumbing between the plugin contract (gRPC,
// see ../../../proto) and the plain objects a plugin works with.
//
//   import { createPlugin } from '@treno-dev/steward-sdk/plugin';
//
//   const plugin = createPlugin({ name: 'github', version: '0.1.0', inputs: [...], validate });
//   plugin.resource({ kind: 'repository', inputs: [...], provision, deprovision, list });
//   plugin.serve();
//
// A plugin is one integration, like a provider. It declares that integration and each of its
// resource or application kinds once, with the handlers attached. The SDK routes every call to the
// right handler by kind, builds the description Steward asks for (including each kind's
// capabilities, which follow from the handlers it has), serves the gRPC services, prints the
// handshake line, answers the health check, fills in whatever a handler leaves out of its response,
// and answers any call without a handler with UNIMPLEMENTED.
//
// Everything a plugin logs must go to stderr: stdout is reserved for the handshake line.
//
// Messages are generated from the contract, which also turns google.protobuf.Struct into plain
// objects, so handlers never deal with protobuf values.

import * as grpc from '@grpc/grpc-js';

import healthCheck from 'grpc-health-check';

import type {
  ApplicationDefinition,
  DeepPartial,
  InputDefinition,
  IntegrationDefinition,
  OutputDefinition,
  PermissionDefinition,
  ResourceDefinition,
  RoleDefinition,
} from '../gen/steward/plugin/v1/plugin.js';
import * as contract from '../gen/steward/plugin/v1/plugin.js';

export * from '../gen/steward/plugin/v1/plugin.js';

const { HealthImplementation } = healthCheck;

// The version of the Steward plugin contract this plugin speaks.
const APP_PROTOCOL_VERSION = 1;

type MaybePromise<T> = T | Promise<T>;

// Protobuf lets a message be absent, so the generated types mark every message field as possibly
// undefined. The runner always sends what a handler needs, and `fill` below makes sure of it, so a
// handler's request has its message fields present. Optional scalars, such as an identity's
// externalId, stay optional because they really can be missing.
type Present<Value> = NonNullable<Value> extends object ? Complete<NonNullable<Value>> : Value;
type Complete<Message> = Message extends (infer Item)[]
  ? Complete<Item>[]
  : Message extends object
    ? { [Field in keyof Message]: Present<Message[Field]> }
    : Message;

// The request and response of each handler, named for what it handles (the entity, then the call),
// for annotating your own handlers. A request has its message fields present, and a response may
// leave fields out.
export type IntegrationValidateRequest = Complete<contract.ValidateRequest>;
export type IntegrationValidateResponse = DeepPartial<contract.ValidateResponse>;
export type IntegrationGrantAccessRequest = Complete<contract.IntegrationServiceGrantAccessRequest>;
export type IntegrationGrantAccessResponse = DeepPartial<contract.IntegrationServiceGrantAccessResponse>;
export type IntegrationRevokeAccessRequest = Complete<contract.IntegrationServiceRevokeAccessRequest>;
export type IntegrationRevokeAccessResponse = DeepPartial<contract.IntegrationServiceRevokeAccessResponse>;
export type IntegrationGetAccessRequest = Complete<contract.IntegrationServiceGetAccessRequest>;
export type IntegrationGetAccessResponse = DeepPartial<contract.IntegrationServiceGetAccessResponse>;
export type ResourceProvisionRequest = Complete<contract.ResourceServiceProvisionRequest>;
export type ResourceProvisionResponse = DeepPartial<contract.ResourceServiceProvisionResponse>;
export type ResourceDeprovisionRequest = Complete<contract.ResourceServiceDeprovisionRequest>;
export type ResourceDeprovisionResponse = DeepPartial<contract.ResourceServiceDeprovisionResponse>;
export type ResourceListRequest = Complete<contract.ResourceServiceListRequest>;
export type ResourceListResponse = DeepPartial<contract.ResourceServiceListResponse>;
export type ResourceGrantAccessRequest = Complete<contract.ResourceServiceGrantAccessRequest>;
export type ResourceGrantAccessResponse = DeepPartial<contract.ResourceServiceGrantAccessResponse>;
export type ResourceRevokeAccessRequest = Complete<contract.ResourceServiceRevokeAccessRequest>;
export type ResourceRevokeAccessResponse = DeepPartial<contract.ResourceServiceRevokeAccessResponse>;
export type ResourceGetAccessRequest = Complete<contract.ResourceServiceGetAccessRequest>;
export type ResourceGetAccessResponse = DeepPartial<contract.ResourceServiceGetAccessResponse>;
export type ApplicationCreateRequest = Complete<contract.ApplicationServiceCreateRequest>;
export type ApplicationCreateResponse = DeepPartial<contract.ApplicationServiceCreateResponse>;
export type ApplicationDeleteRequest = Complete<contract.ApplicationServiceDeleteRequest>;
export type ApplicationDeleteResponse = DeepPartial<contract.ApplicationServiceDeleteResponse>;
export type ApplicationDeployRequest = Complete<contract.ApplicationServiceDeployRequest>;
export type ApplicationDeployResponse = DeepPartial<contract.ApplicationServiceDeployResponse>;
export type ApplicationSetVariablesRequest = Complete<contract.ApplicationServiceSetVariablesRequest>;
export type ApplicationSetVariablesResponse = DeepPartial<contract.ApplicationServiceSetVariablesResponse>;
export type ApplicationListRequest = Complete<contract.ApplicationServiceListRequest>;
export type ApplicationListResponse = DeepPartial<contract.ApplicationServiceListResponse>;

// For a generated service definition, the handler of each call: it takes the request and returns
// the response, which may leave fields out.
export type Handlers<Service> = {
  [Call in keyof Service]?: Service[Call] extends {
    requestDeserialize: (value: Buffer) => infer Request;
    responseDeserialize: (value: Buffer) => infer Response;
  }
    ? (request: Complete<Request>) => MaybePromise<DeepPartial<Response>>
    : never;
};

type Describe = {
  title?: string;
  description?: string;
  inputs?: DeepPartial<InputDefinition>[];
};

type Access = {
  permissions?: DeepPartial<PermissionDefinition>[];
  roles?: DeepPartial<RoleDefinition>[];
};

// The plugin and its integration: what is needed to create an integration from it, and, when the
// integration supports access as a whole, the handlers for that.
export type PluginOptions = Describe &
  Access & {
    name: string;
    version: string;
    validate?: Handlers<typeof contract.PluginServiceService>['validate'];
  } & Handlers<typeof contract.IntegrationServiceService>;

export type ResourceOptions = Describe &
  Access & {
    kind: string;
    outputs?: DeepPartial<OutputDefinition>[];
  } & Handlers<typeof contract.ResourceServiceService>;

export type ApplicationOptions = Describe & {
  kind: string;
  sources?: string[];
  outputs?: DeepPartial<OutputDefinition>[];
} & Handlers<typeof contract.ApplicationServiceService>;

// A request carries the kind or the thing it is about.
type AnyRequest = {
  kind?: string;
  resource?: { kind: string };
  application?: { kind: string };
};

type Target = 'integration' | 'resource' | 'application';

const ACCESS_CALLS = ['grantAccess', 'revokeAccess', 'getAccess'] as const;
const PROVISIONING_CALLS = ['provision', 'deprovision'] as const;

function has(options: object, calls: readonly string[]): boolean {
  return calls.some((call) => typeof (options as Record<string, unknown>)[call] === 'function');
}

function fail(code: grpc.status, message: string): never {
  throw { code, message };
}

function resourceDefinition(options: ResourceOptions): DeepPartial<ResourceDefinition> {
  const { kind, title, description, inputs, outputs, permissions, roles } = options;
  const capabilities: contract.ResourceCapability[] = [];

  if (has(options, ACCESS_CALLS)) {
    capabilities.push(contract.ResourceCapability.RESOURCE_CAPABILITY_ACCESS);
  }

  if (has(options, PROVISIONING_CALLS)) {
    capabilities.push(contract.ResourceCapability.RESOURCE_CAPABILITY_PROVISIONING);
  }

  if (has(options, ['list'])) {
    capabilities.push(contract.ResourceCapability.RESOURCE_CAPABILITY_DISCOVERY);
  }

  return { kind, title, description, inputs, outputs, permissions, roles, capabilities };
}

function applicationDefinition(options: ApplicationOptions): DeepPartial<ApplicationDefinition> {
  const { kind, title, description, inputs, sources, outputs } = options;

  return { kind, title, description, inputs, sources, outputs };
}

export class Plugin {
  readonly resources = new Map<string, ResourceOptions>();
  readonly applications = new Map<string, ApplicationOptions>();

  constructor(readonly options: PluginOptions) {}

  /** Declares a kind of resource the integration manages, with its handlers. */
  resource(options: ResourceOptions): this {
    this.resources.set(options.kind, options);

    return this;
  }

  /** Declares a kind of application the integration runs, with its handlers. */
  application(options: ApplicationOptions): this {
    this.applications.set(options.kind, options);

    return this;
  }

  /** Starts serving the plugin. The runner stops it when it is done. */
  serve(): void {
    serve(this);
  }

  describe() {
    const { name, version } = this.options;

    return { name, version, integration: this.integrationDefinition() };
  }

  // Routes a call to the handler of the same name on whatever the request is about: the
  // integration itself, or one of its kinds of resource or application.
  route(call: string, target: Target, kindOf: (request: AnyRequest) => string = () => '') {
    return (request: AnyRequest) => {
      const options = target === 'integration' ? this.options : this.kind(target, kindOf(request));
      const handler = (options as Record<string, unknown>)[call] as ((request: AnyRequest) => unknown) | undefined;

      return handler
        ? handler(request)
        : fail(grpc.status.UNIMPLEMENTED, `${call} is not implemented for this ${target}`);
    };
  }

  private integrationDefinition(): DeepPartial<IntegrationDefinition> {
    const { title, description, inputs, permissions, roles } = this.options;
    const access = has(this.options, ACCESS_CALLS);

    return {
      title,
      description,
      inputs,
      permissions,
      roles,
      capabilities: access ? [contract.IntegrationCapability.INTEGRATION_CAPABILITY_ACCESS] : [],
      resources: [...this.resources.values()].map(resourceDefinition),
      applications: [...this.applications.values()].map(applicationDefinition),
    };
  }

  private kind(target: 'resource' | 'application', kind: string): object {
    const kinds = target === 'resource' ? this.resources : this.applications;

    return kinds.get(kind) ?? fail(grpc.status.NOT_FOUND, `this plugin has no ${target} "${kind}"`);
  }
}

/** Creates a plugin, which is one integration. Declare its resources, then call `serve()`. */
export function createPlugin(options: PluginOptions): Plugin {
  return new Plugin(options);
}

// Serving --------------------------------------------------------------------------------------

type Registered = Record<string, { path: string }>;
type Routed = Record<string, (request: AnyRequest) => unknown>;
type MessageType = { fromPartial(object: unknown): unknown };

// A call's response type is named after the call, optionally prefixed with its service, which is
// the naming the contract's lint enforces: DescribeResponse, ResourceServiceProvisionResponse.
function responseType(path: string): MessageType {
  const [, service, method] = /\.([A-Za-z]+)\/([A-Za-z]+)$/.exec(path) ?? [];
  const messages = contract as unknown as Record<string, MessageType | undefined>;
  const type = messages[`${method}Response`] ?? messages[`${service}${method}Response`];

  if (!type) {
    throw new Error(`no response type found for ${path}`);
  }

  return type;
}

function toStatus(error: unknown): grpc.ServiceError {
  if (typeof error === 'object' && error !== null && typeof (error as { code?: unknown }).code === 'number') {
    return error as grpc.ServiceError;
  }

  const message = error instanceof Error ? error.message : String(error);

  return { code: grpc.status.INTERNAL, message } as grpc.ServiceError;
}

// Makes the message fields a handler relies on present, so it can read `integration.inputs.x`
// without checking. An absent message becomes the generated type's default, and an absent Struct
// becomes an empty object.
const MESSAGES: Record<string, MessageType> = {
  integration: contract.Integration,
  resource: contract.Resource,
  application: contract.Application,
  identity: contract.Identity,
  role: contract.Role,
  source: contract.ApplicationSource,
};
const STRUCTS = ['inputs', 'outputs', 'attrs'];

function fill(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(fill);
  }

  if (typeof value !== 'object' || value === null) {
    return value;
  }

  const message = value as Record<string, unknown>;

  for (const [field, item] of Object.entries(message)) {
    const absent = item === undefined || item === null;

    if (absent && field in MESSAGES) {
      message[field] = fill(MESSAGES[field].fromPartial({}));
      continue;
    }

    message[field] = absent && STRUCTS.includes(field) ? {} : fill(item);
  }

  return message;
}

function unary(path: string, handler: (request: AnyRequest) => unknown): grpc.handleUnaryCall<unknown, unknown> {
  const response = responseType(path);

  return (call, callback) => {
    Promise.resolve()
      .then(() => handler(fill(call.request) as AnyRequest))
      .then((result) => callback(null, response.fromPartial(result ?? {})))
      .catch((error) => callback(toStatus(error)));
  };
}

function unimplemented(path: string): grpc.handleUnaryCall<unknown, unknown> {
  return (_call, callback) =>
    callback({
      code: grpc.status.UNIMPLEMENTED,
      message: `${path} is not implemented by this plugin`,
    } as grpc.ServiceError);
}

// Maps each routed call onto the service call of the same name, and answers any call without a
// route with UNIMPLEMENTED.
function implement(service: Registered, routed: Routed): grpc.UntypedServiceImplementation {
  const implementation: grpc.UntypedServiceImplementation = {};

  for (const [name, definition] of Object.entries(service)) {
    const handler = routed[name];

    implementation[name] = handler ? unary(definition.path, handler) : unimplemented(definition.path);
  }

  return implementation;
}

function routes(plugin: Plugin, calls: readonly string[], target: Target, kindOf?: (request: AnyRequest) => string): Routed {
  return Object.fromEntries(calls.map((call) => [call, plugin.route(call, target, kindOf)]));
}

function serve(plugin: Plugin): void {
  const server = new grpc.Server();
  const anyResource = plugin.resources.size > 0;
  const anyApplication = plugin.applications.size > 0;
  const anyAccess = has(plugin.options, ACCESS_CALLS);

  server.addService(contract.PluginServiceService, implement(contract.PluginServiceService, {
    describe: () => plugin.describe(),
    validate: plugin.route('validate', 'integration'),
  }));

  if (anyAccess) {
    server.addService(
      contract.IntegrationServiceService as unknown as grpc.ServiceDefinition,
      implement(contract.IntegrationServiceService, routes(plugin, ACCESS_CALLS, 'integration')),
    );
  }

  if (anyResource) {
    const resourceKind = (request: AnyRequest) => request.resource?.kind ?? request.kind ?? '';

    server.addService(
      contract.ResourceServiceService as unknown as grpc.ServiceDefinition,
      implement(
        contract.ResourceServiceService,
        routes(plugin, [...ACCESS_CALLS, ...PROVISIONING_CALLS, 'list'], 'resource', resourceKind),
      ),
    );
  }

  if (anyApplication) {
    const applicationKind = (request: AnyRequest) => request.application?.kind ?? request.kind ?? '';
    const calls = ['create', 'delete', 'deploy', 'setVariables', 'list'];

    server.addService(
      contract.ApplicationServiceService as unknown as grpc.ServiceDefinition,
      implement(contract.ApplicationServiceService, routes(plugin, calls, 'application', applicationKind)),
    );
  }

  new HealthImplementation({ plugin: 'SERVING' }).addToServer(server);

  server.bindAsync('127.0.0.1:0', grpc.ServerCredentials.createInsecure(), (error, port) => {
    if (error) {
      console.error(error.message);
      process.exit(1);
    }

    // CORE-PROTOCOL-VERSION | APP-PROTOCOL-VERSION | NETWORK-TYPE | NETWORK-ADDR | PROTOCOL
    console.log(`1|${APP_PROTOCOL_VERSION}|tcp|127.0.0.1:${port}|grpc`);
  });

  for (const signal of ['SIGINT', 'SIGTERM'] as const) {
    process.on(signal, () => server.tryShutdown(() => process.exit(0)));
  }
}
