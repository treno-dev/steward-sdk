from __future__ import annotations

import asyncio
import inspect
import os
import secrets
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

from steward_sdk._gen.steward.plugin.v1 import plugin_pb2 as contract

# The version of the Steward plugin contract this plugin speaks.
APP_PROTOCOL_VERSION = 1

# A runner sets this cookie in the environment of every plugin it starts, as hashicorp/go-plugin does.
# A plugin that does not find it was started by hand. Kept in step with go/plugin/handshake.go.
MAGIC_COOKIE_KEY = "STEWARD_PLUGIN"
MAGIC_COOKIE_VALUE = "b6d7a1f2-steward-plugin"

# The request of each handler, named for what it handles (the entity, then the call), for annotating
# your own handlers. They are the generated messages: fields are snake_case, a message that was not
# sent reads as empty, and a Struct (such as `inputs`) supports `[]`, `in` and `dict()`.
PluginValidateRequest = contract.ValidateRequest
PluginGrantAccessRequest = contract.PluginServiceGrantAccessRequest
PluginRevokeAccessRequest = contract.PluginServiceRevokeAccessRequest
PluginGetAccessRequest = contract.PluginServiceGetAccessRequest
ResourceProvisionRequest = contract.ResourceServiceProvisionRequest
ResourceDeprovisionRequest = contract.ResourceServiceDeprovisionRequest
ResourceListRequest = contract.ResourceServiceListRequest
ResourceGrantAccessRequest = contract.ResourceServiceGrantAccessRequest
ResourceRevokeAccessRequest = contract.ResourceServiceRevokeAccessRequest
ResourceGetAccessRequest = contract.ResourceServiceGetAccessRequest
ApplicationCreateRequest = contract.ApplicationServiceCreateRequest
ApplicationDestroyRequest = contract.ApplicationServiceDestroyRequest
ApplicationDeployRequest = contract.ApplicationServiceDeployRequest

# A handler returns the generated response message, or a plain dict with the fields it has
# something to say about (names may be snake_case or camelCase), or nothing at all.
PluginValidateResponse = Union[contract.ValidateResponse, Mapping[str, Any], None]
PluginGrantAccessResponse = Union[contract.PluginServiceGrantAccessResponse, Mapping[str, Any], None]
PluginRevokeAccessResponse = Union[contract.PluginServiceRevokeAccessResponse, Mapping[str, Any], None]
PluginGetAccessResponse = Union[contract.PluginServiceGetAccessResponse, Mapping[str, Any], None]
ResourceProvisionResponse = Union[contract.ResourceServiceProvisionResponse, Mapping[str, Any], None]
ResourceDeprovisionResponse = Union[contract.ResourceServiceDeprovisionResponse, Mapping[str, Any], None]
ResourceListResponse = Union[contract.ResourceServiceListResponse, Mapping[str, Any], None]
ResourceGrantAccessResponse = Union[contract.ResourceServiceGrantAccessResponse, Mapping[str, Any], None]
ResourceRevokeAccessResponse = Union[contract.ResourceServiceRevokeAccessResponse, Mapping[str, Any], None]
ResourceGetAccessResponse = Union[contract.ResourceServiceGetAccessResponse, Mapping[str, Any], None]
ApplicationCreateResponse = Union[contract.ApplicationServiceCreateResponse, Mapping[str, Any], None]
ApplicationDestroyResponse = Union[contract.ApplicationServiceDestroyResponse, Mapping[str, Any], None]
ApplicationDeployResponse = Union[contract.ApplicationServiceDeployResponse, Mapping[str, Any], None]

# A handler takes the request and returns the response. It may be sync or async.
Handler = Callable[[Any], Union[Any, Awaitable[Any]]]
Definitions = Sequence[Mapping[str, Any]]

ACCESS_CALLS = ("grant_access", "revoke_access", "get_access")
PROVISIONING_CALLS = ("provision", "deprovision")
RESOURCE_CALLS = (*ACCESS_CALLS, *PROVISIONING_CALLS, "list")
APPLICATION_CALLS = ("create", "destroy", "deploy")


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
    """A plugin. Declare its resources, then call `serve()`."""

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
        secrets: Definitions = (),
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
        """Declares a kind of resource the plugin manages, with its handlers."""
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
            secrets=secrets,
            outputs=outputs,
            permissions=permissions,
            roles=roles,
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
        secrets: Definitions = (),
        outputs: Definitions = (),
        sources: Sequence[str] = (),
        create: Handler | None = None,
        destroy: Handler | None = None,
        deploy: Handler | None = None,
    ) -> Plugin:
        """Declares a kind of application the plugin runs, with its handlers."""
        handlers = _handlers(create=create, destroy=destroy, deploy=deploy)
        definition = _defined(
            kind=kind,
            title=title,
            description=description,
            inputs=inputs,
            secrets=secrets,
            outputs=outputs,
            sources=sources,
        )
        self.applications[kind] = _Kind(kind, definition, handlers)

        return self

    def describe(self) -> contract.DescribeResponse:
        definition = {
            **self.definition,
            "resources": [kind.definition for kind in self.resources.values()],
            "applications": [kind.definition for kind in self.applications.values()],
        }

        return json_format.ParseDict(
            {"name": self.name, "version": self.version, "definition": definition},
            contract.DescribeResponse(),
        )

    def serve(self) -> None:
        """Starts serving the plugin. The runner stops it when it is done.

        A plugin is started by a runner, which sets the environment this checks. Started by hand, it says
        so and exits, so try it with `steward plugin validate` instead.
        """
        if os.environ.get(MAGIC_COOKIE_KEY) != MAGIC_COOKIE_VALUE:
            print("This program is a Steward plugin. It is started by a runner, not run directly.", file=sys.stderr)
            print("To try it, use `steward plugin validate` or `steward plugin run`.", file=sys.stderr)
            sys.exit(1)

        asyncio.run(_serve(self))


def create_plugin(
    *,
    name: str,
    version: str,
    title: str = "",
    description: str = "",
    inputs: Definitions = (),
    secrets: Definitions = (),
    permissions: Definitions = (),
    roles: Definitions = (),
    validate: Handler | None = None,
    grant_access: Handler | None = None,
    revoke_access: Handler | None = None,
    get_access: Handler | None = None,
) -> Plugin:
    """Creates a plugin. Declare its resources, then call `serve()`."""
    definition = _defined(
        title=title,
        description=description,
        inputs=inputs,
        secrets=secrets,
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
    if target == "plugin":
        return plugin.handlers

    kinds = plugin.resources if target == "resource" else plugin.applications
    kind = _kind_of(request, target)

    if kind not in kinds:
        raise PluginError(StatusCode.NOT_FOUND, f'this plugin has no {target} "{kind}"')

    return kinds[kind].handlers


def _handler(descriptor: Any, routed: Callable[[Message], Awaitable[Any]]) -> grpc.RpcMethodHandler:
    request = message_factory.GetMessageClass(descriptor.input_type)
    response = message_factory.GetMessageClass(descriptor.output_type)

    async def method(message: Message, context: grpc.aio.ServicerContext) -> Message:
        try:
            return _response(await routed(message), response)
        except PluginError as error:
            await context.abort(error.code, error.message)
        except Exception as error:
            traceback.print_exc(file=sys.stderr)
            await context.abort(StatusCode.INTERNAL, str(error))

        raise AssertionError("unreachable")

    return grpc.unary_unary_rpc_method_handler(
        method,
        request_deserializer=request.FromString,
        response_serializer=response.SerializeToString,
    )


# Serves a service from the generated descriptor, with a handler for each routed call. A call without one
# is not registered, and gRPC itself answers UNIMPLEMENTED, as the generated servicer would. Doing this
# here means only the messages are generated, so no generated code has to be patched.
def _register(server: grpc.aio.Server, service: str, routes: Mapping[str, Callable[[Message], Awaitable[Any]]]) -> None:
    descriptor = contract.DESCRIPTOR.services_by_name[service]
    handlers = {
        _pascal(call): _handler(descriptor.methods_by_name[_pascal(call)], routed) for call, routed in routes.items()
    }

    server.add_generic_rpc_handlers((grpc.method_handlers_generic_handler(descriptor.full_name, handlers),))


def _routes(plugin: Plugin, calls: Sequence[str], target: str) -> dict[str, Callable[[Message], Awaitable[Any]]]:
    return {call: _route(plugin, call, target) for call in calls}


def _listen() -> tuple[str, str]:
    """Where to listen, and what to announce: a unix socket in the directory the runner made for it, when
    it set one, which only the runner's user can open; otherwise a port on the loopback interface, which
    is announced once it is known."""
    directory = os.environ.get("PLUGIN_UNIX_SOCKET_DIR")

    if directory and sys.platform != "win32":
        path = os.path.join(directory, f"plugin-{secrets.token_hex(6)}")

        return f"unix:{path}", f"unix|{path}"

    return "127.0.0.1:0", ""


def _shutdown(server: grpc.aio.Server) -> grpc.GenericRpcHandler:
    """Serves the call go-plugin makes to ask a plugin to shut down, so that it exits at once and the
    runner does not wait for it. It has no generated code, since its messages are empty."""

    async def shutdown(_request: bytes, _context: grpc.aio.ServicerContext) -> bytes:
        asyncio.ensure_future(server.stop(grace=1))

        return b""

    return grpc.method_handlers_generic_handler(
        "plugin.GRPCController",
        {"Shutdown": grpc.unary_unary_rpc_method_handler(shutdown)},
    )


async def _serve(plugin: Plugin) -> None:
    server = grpc.aio.server()

    async def describe(_request: Message) -> Message:
        return plugin.describe()

    # Every plugin must answer Validate; without a handler of its own, every integration is accepted.
    async def accept(_request: Message) -> None:
        return None

    validate = _route(plugin, "validate", "plugin") if "validate" in plugin.handlers else accept

    _register(
        server,
        "PluginService",
        {"describe": describe, "validate": validate, **_routes(plugin, ACCESS_CALLS, "plugin")},
    )

    if plugin.resources:
        _register(server, "ResourceService", _routes(plugin, RESOURCE_CALLS, "resource"))

    if plugin.applications:
        _register(server, "ApplicationService", _routes(plugin, APPLICATION_CALLS, "application"))

    status = health.aio.HealthServicer()
    health_pb2_grpc.add_HealthServicer_to_server(status, server)
    await status.set("plugin", health_pb2.HealthCheckResponse.SERVING)

    server.add_generic_rpc_handlers((_shutdown(server),))

    bind, announced = _listen()
    port = server.add_insecure_port(bind)
    await server.start()

    # CORE-PROTOCOL-VERSION | APP-PROTOCOL-VERSION | NETWORK-TYPE | NETWORK-ADDR | PROTOCOL
    print(f"1|{APP_PROTOCOL_VERSION}|{announced or f'tcp|127.0.0.1:{port}'}|grpc", flush=True)

    loop = asyncio.get_running_loop()

    for name in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(name, lambda: asyncio.ensure_future(server.stop(grace=1)))

    await server.wait_for_termination()
