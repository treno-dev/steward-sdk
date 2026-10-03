from __future__ import annotations

import asyncio
import inspect
import signal
import sys
import traceback
from collections.abc import Awaitable, Callable, Mapping, Sequence
from dataclasses import dataclass, field
from typing import Any, Union

import grpc
from google.protobuf import json_format, message_factory
from google.protobuf.message import Message
from grpc import StatusCode
from grpc_health.v1 import health, health_pb2, health_pb2_grpc

from steward_plugin._gen.steward.plugin.v1 import plugin_pb2 as contract
from steward_plugin._gen.steward.plugin.v1 import plugin_pb2_grpc as services

# The version of the Steward plugin contract this plugin speaks.
APP_PROTOCOL_VERSION = 1

# The request of each handler, named for what it handles (the entity, then the call), for annotating
# your own handlers. They are the generated messages: fields are snake_case, a message that was not
# sent reads as empty, and a Struct (such as `inputs`) supports `[]`, `in` and `dict()`.
IntegrationValidateRequest = contract.ValidateRequest
IntegrationGrantAccessRequest = contract.IntegrationServiceGrantAccessRequest
IntegrationRevokeAccessRequest = contract.IntegrationServiceRevokeAccessRequest
IntegrationGetAccessRequest = contract.IntegrationServiceGetAccessRequest
ResourceProvisionRequest = contract.ResourceServiceProvisionRequest
ResourceDeprovisionRequest = contract.ResourceServiceDeprovisionRequest
ResourceListRequest = contract.ResourceServiceListRequest
ResourceGrantAccessRequest = contract.ResourceServiceGrantAccessRequest
ResourceRevokeAccessRequest = contract.ResourceServiceRevokeAccessRequest
ResourceGetAccessRequest = contract.ResourceServiceGetAccessRequest
ApplicationCreateRequest = contract.ApplicationServiceCreateRequest
ApplicationDeleteRequest = contract.ApplicationServiceDeleteRequest
ApplicationDeployRequest = contract.ApplicationServiceDeployRequest
ApplicationSetVariablesRequest = contract.ApplicationServiceSetVariablesRequest
ApplicationListRequest = contract.ApplicationServiceListRequest

# A handler returns the generated response message, or a plain dict with the fields it has
# something to say about (names may be snake_case or camelCase), or nothing at all.
IntegrationValidateResponse = Union[contract.ValidateResponse, Mapping[str, Any], None]
IntegrationGrantAccessResponse = Union[contract.IntegrationServiceGrantAccessResponse, Mapping[str, Any], None]
IntegrationRevokeAccessResponse = Union[contract.IntegrationServiceRevokeAccessResponse, Mapping[str, Any], None]
IntegrationGetAccessResponse = Union[contract.IntegrationServiceGetAccessResponse, Mapping[str, Any], None]
ResourceProvisionResponse = Union[contract.ResourceServiceProvisionResponse, Mapping[str, Any], None]
ResourceDeprovisionResponse = Union[contract.ResourceServiceDeprovisionResponse, Mapping[str, Any], None]
ResourceListResponse = Union[contract.ResourceServiceListResponse, Mapping[str, Any], None]
ResourceGrantAccessResponse = Union[contract.ResourceServiceGrantAccessResponse, Mapping[str, Any], None]
ResourceRevokeAccessResponse = Union[contract.ResourceServiceRevokeAccessResponse, Mapping[str, Any], None]
ResourceGetAccessResponse = Union[contract.ResourceServiceGetAccessResponse, Mapping[str, Any], None]
ApplicationCreateResponse = Union[contract.ApplicationServiceCreateResponse, Mapping[str, Any], None]
ApplicationDeleteResponse = Union[contract.ApplicationServiceDeleteResponse, Mapping[str, Any], None]
ApplicationDeployResponse = Union[contract.ApplicationServiceDeployResponse, Mapping[str, Any], None]
ApplicationSetVariablesResponse = Union[contract.ApplicationServiceSetVariablesResponse, Mapping[str, Any], None]
ApplicationListResponse = Union[contract.ApplicationServiceListResponse, Mapping[str, Any], None]

# A handler takes the request and returns the response. It may be sync or async.
Handler = Callable[[Any], Union[Any, Awaitable[Any]]]
Definitions = Sequence[Mapping[str, Any]]

ACCESS_CALLS = ("grant_access", "revoke_access", "get_access")
PROVISIONING_CALLS = ("provision", "deprovision")
RESOURCE_CALLS = (*ACCESS_CALLS, *PROVISIONING_CALLS, "list")
APPLICATION_CALLS = ("create", "delete", "deploy", "set_variables", "list")


class PluginError(Exception):
    """Fails a call with a specific gRPC status, such as FAILED_PRECONDITION or NOT_FOUND.

    Any other exception a handler raises is reported as INTERNAL.
    """

    def __init__(self, code: StatusCode, message: str) -> None:
        super().__init__(message)
        self.code = code
        self.message = message


def plain(message: Message) -> dict[str, Any]:
    """A message, such as an input Struct or a role, as a plain dict with snake_case keys."""
    return json_format.MessageToDict(message, preserving_proto_field_name=True)


@dataclass
class _Kind:
    kind: str
    definition: dict[str, Any]
    handlers: dict[str, Handler] = field(default_factory=dict)


def _defined(**values: Any) -> dict[str, Any]:
    return {name: value for name, value in values.items() if value not in (None, "", (), [])}


def _handlers(**calls: Handler | None) -> dict[str, Handler]:
    return {call: handler for call, handler in calls.items() if handler is not None}


class Plugin:
    """A plugin, which is one integration. Declare its resources, then call `serve()`."""

    def __init__(self, name: str, version: str, definition: dict[str, Any], handlers: dict[str, Handler]) -> None:
        self.name = name
        self.version = version
        self.definition = definition
        self.handlers = handlers
        self.resources: dict[str, _Kind] = {}
        self.applications: dict[str, _Kind] = {}

    def resource(
        self,
        *,
        kind: str,
        title: str = "",
        description: str = "",
        inputs: Definitions = (),
        outputs: Definitions = (),
        permissions: Definitions = (),
        roles: Definitions = (),
        provision: Handler | None = None,
        deprovision: Handler | None = None,
        list: Handler | None = None,
        grant_access: Handler | None = None,
        revoke_access: Handler | None = None,
        get_access: Handler | None = None,
    ) -> Plugin:
        """Declares a kind of resource the integration manages, with its handlers."""
        handlers = _handlers(
            provision=provision,
            deprovision=deprovision,
            list=list,
            grant_access=grant_access,
            revoke_access=revoke_access,
            get_access=get_access,
        )
        definition = _defined(
            kind=kind,
            title=title,
            description=description,
            inputs=inputs,
            outputs=outputs,
            permissions=permissions,
            roles=roles,
            capabilities=_resource_capabilities(handlers),
        )
        self.resources[kind] = _Kind(kind, definition, handlers)

        return self

    def application(
        self,
        *,
        kind: str,
        title: str = "",
        description: str = "",
        inputs: Definitions = (),
        outputs: Definitions = (),
        sources: Sequence[str] = (),
        create: Handler | None = None,
        delete: Handler | None = None,
        deploy: Handler | None = None,
        set_variables: Handler | None = None,
        list: Handler | None = None,
    ) -> Plugin:
        """Declares a kind of application the integration runs, with its handlers."""
        handlers = _handlers(create=create, delete=delete, deploy=deploy, set_variables=set_variables, list=list)
        definition = _defined(
            kind=kind,
            title=title,
            description=description,
            inputs=inputs,
            outputs=outputs,
            sources=sources,
        )
        self.applications[kind] = _Kind(kind, definition, handlers)

        return self

    def describe(self) -> contract.DescribeResponse:
        access = any(call in self.handlers for call in ACCESS_CALLS)
        integration = {
            **self.definition,
            "capabilities": ["INTEGRATION_CAPABILITY_ACCESS"] if access else [],
            "resources": [kind.definition for kind in self.resources.values()],
            "applications": [kind.definition for kind in self.applications.values()],
        }

        return json_format.ParseDict(
            {"name": self.name, "version": self.version, "integration": integration},
            contract.DescribeResponse(),
        )

    def serve(self) -> None:
        """Starts serving the plugin. The runner stops it when it is done."""
        asyncio.run(_serve(self))


def create_plugin(
    *,
    name: str,
    version: str,
    title: str = "",
    description: str = "",
    inputs: Definitions = (),
    permissions: Definitions = (),
    roles: Definitions = (),
    validate: Handler | None = None,
    grant_access: Handler | None = None,
    revoke_access: Handler | None = None,
    get_access: Handler | None = None,
) -> Plugin:
    """Creates a plugin, which is one integration. Declare its resources, then call `serve()`."""
    definition = _defined(
        title=title,
        description=description,
        inputs=inputs,
        permissions=permissions,
        roles=roles,
    )
    handlers = _handlers(
        validate=validate,
        grant_access=grant_access,
        revoke_access=revoke_access,
        get_access=get_access,
    )

    return Plugin(name, version, definition, handlers)


def _resource_capabilities(handlers: Mapping[str, Handler]) -> list[str]:
    capabilities = []

    if any(call in handlers for call in ACCESS_CALLS):
        capabilities.append("RESOURCE_CAPABILITY_ACCESS")

    if any(call in handlers for call in PROVISIONING_CALLS):
        capabilities.append("RESOURCE_CAPABILITY_PROVISIONING")

    if "list" in handlers:
        capabilities.append("RESOURCE_CAPABILITY_DISCOVERY")

    return capabilities


# Serving --------------------------------------------------------------------------------------


def _pascal(call: str) -> str:
    return "".join(word.capitalize() for word in call.split("_"))


def _kind_of(request: Message, target: str) -> str:
    if target in request.DESCRIPTOR.fields_by_name:
        return getattr(request, target).kind

    return request.kind


def _to_plain(value: Any) -> Any:
    if isinstance(value, Message):
        return plain(value)

    if isinstance(value, Mapping):
        return {key: _to_plain(item) for key, item in value.items()}

    if isinstance(value, (list, tuple)):
        return [_to_plain(item) for item in value]

    return value


def _response(result: Any, response: type[Message]) -> Message:
    if isinstance(result, response):
        return result

    if result is None:
        return response()

    return json_format.ParseDict(_to_plain(result), response())


async def _invoke(handler: Handler, request: Message) -> Any:
    if inspect.iscoroutinefunction(handler):
        return await handler(request)

    result = await asyncio.to_thread(handler, request)

    return await result if inspect.isawaitable(result) else result


def _route(plugin: Plugin, call: str, target: str) -> Callable[[Message], Awaitable[Any]]:
    async def routed(request: Message) -> Any:
        owner = _owner(plugin, target, request)
        handler = owner.get(call)

        if handler is None:
            raise PluginError(StatusCode.UNIMPLEMENTED, f"{call} is not implemented for this {target}")

        return await _invoke(handler, request)

    return routed


def _owner(plugin: Plugin, target: str, request: Message) -> Mapping[str, Handler]:
    if target == "integration":
        return plugin.handlers

    kinds = plugin.resources if target == "resource" else plugin.applications
    kind = _kind_of(request, target)

    if kind not in kinds:
        raise PluginError(StatusCode.NOT_FOUND, f'this plugin has no {target} "{kind}"')

    return kinds[kind].handlers


def _method(descriptor: Any, routed: Callable[[Message], Awaitable[Any]]) -> Callable[..., Awaitable[Message]]:
    response = message_factory.GetMessageClass(descriptor.output_type)

    async def method(self: Any, request: Message, context: grpc.aio.ServicerContext) -> Message:
        try:
            return _response(await routed(request), response)
        except PluginError as error:
            await context.abort(error.code, error.message)
        except Exception as error:
            traceback.print_exc(file=sys.stderr)
            await context.abort(StatusCode.INTERNAL, str(error))

        raise AssertionError("unreachable")

    return method


# Builds the servicer for a service: each routed call is the method of the same name, and anything
# else keeps the generated default, which answers UNIMPLEMENTED.
def _servicer(service: str, base: type, routes: Mapping[str, Callable[[Message], Awaitable[Any]]]) -> Any:
    descriptor = contract.DESCRIPTOR.services_by_name[service]
    methods = {
        name: _method(descriptor.methods_by_name[name], routed)
        for name, routed in ((_pascal(call), routed) for call, routed in routes.items())
    }

    return type(f"{service}Servicer", (base,), methods)()


def _routes(plugin: Plugin, calls: Sequence[str], target: str) -> dict[str, Callable[[Message], Awaitable[Any]]]:
    return {call: _route(plugin, call, target) for call in calls}


async def _serve(plugin: Plugin) -> None:
    server = grpc.aio.server()

    async def describe(_request: Message) -> Message:
        return plugin.describe()

    services.add_PluginServiceServicer_to_server(
        _servicer(
            "PluginService",
            services.PluginServiceServicer,
            {"describe": describe, "validate": _route(plugin, "validate", "integration")},
        ),
        server,
    )

    if any(call in plugin.handlers for call in ACCESS_CALLS):
        services.add_IntegrationServiceServicer_to_server(
            _servicer("IntegrationService", services.IntegrationServiceServicer, _routes(plugin, ACCESS_CALLS, "integration")),
            server,
        )

    if plugin.resources:
        services.add_ResourceServiceServicer_to_server(
            _servicer("ResourceService", services.ResourceServiceServicer, _routes(plugin, RESOURCE_CALLS, "resource")),
            server,
        )

    if plugin.applications:
        services.add_ApplicationServiceServicer_to_server(
            _servicer("ApplicationService", services.ApplicationServiceServicer, _routes(plugin, APPLICATION_CALLS, "application")),
            server,
        )

    status = health.aio.HealthServicer()
    health_pb2_grpc.add_HealthServicer_to_server(status, server)
    await status.set("plugin", health_pb2.HealthCheckResponse.SERVING)

    port = server.add_insecure_port("127.0.0.1:0")
    await server.start()

    # CORE-PROTOCOL-VERSION | APP-PROTOCOL-VERSION | NETWORK-TYPE | NETWORK-ADDR | PROTOCOL
    print(f"1|{APP_PROTOCOL_VERSION}|tcp|127.0.0.1:{port}|grpc", flush=True)

    loop = asyncio.get_running_loop()

    for name in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(name, lambda: asyncio.ensure_future(server.stop(grace=1)))

    await server.wait_for_termination()
