"""The plugin part of the Steward SDK for Python: the plumbing between the plugin contract (gRPC,
see ../proto) and the functions a plugin writes.

    from steward_sdk.plugin import create_plugin


    plugin = create_plugin(name="github", version="0.1.0", inputs=[...], validate=validate)
    plugin.resource(kind="repository", inputs=[...], provision=provision, deprovision=deprovision)
    plugin.serve()

A plugin declares itself and each of its resource or application kinds once, with the handlers
attached. The SDK routes every call to the right handler by kind, builds the description Steward asks
for, serves the gRPC services, prints the handshake line, answers the health check, and answers any
call without a handler with UNIMPLEMENTED.

Everything a plugin logs must go to stderr: stdout is reserved for the handshake line.
"""

from steward_sdk._gen.steward.plugin.v1.plugin_pb2 import *  # noqa: F401,F403
from steward_sdk.plugin._plugin import (
    ApplicationCreateRequest,
    ApplicationCreateResponse,
    ApplicationDeleteRequest,
    ApplicationDeleteResponse,
    ApplicationDeployRequest,
    ApplicationDeployResponse,
    ApplicationListRequest,
    ApplicationListResponse,
    ApplicationSetVariablesRequest,
    ApplicationSetVariablesResponse,
    Plugin,
    PluginError,
    PluginGetAccessRequest,
    PluginGetAccessResponse,
    PluginGrantAccessRequest,
    PluginGrantAccessResponse,
    PluginRevokeAccessRequest,
    PluginRevokeAccessResponse,
    PluginValidateRequest,
    PluginValidateResponse,
    ResourceDeprovisionRequest,
    ResourceDeprovisionResponse,
    ResourceGetAccessRequest,
    ResourceGetAccessResponse,
    ResourceGrantAccessRequest,
    ResourceGrantAccessResponse,
    ResourceListRequest,
    ResourceListResponse,
    ResourceProvisionRequest,
    ResourceProvisionResponse,
    ResourceRevokeAccessRequest,
    ResourceRevokeAccessResponse,
    StatusCode,
    create_plugin,
    plain,
)

__all__ = [
    "ApplicationCreateRequest",
    "ApplicationCreateResponse",
    "ApplicationDeleteRequest",
    "ApplicationDeleteResponse",
    "ApplicationDeployRequest",
    "ApplicationDeployResponse",
    "ApplicationListRequest",
    "ApplicationListResponse",
    "ApplicationSetVariablesRequest",
    "ApplicationSetVariablesResponse",
    "Plugin",
    "PluginError",
    "PluginGetAccessRequest",
    "PluginGetAccessResponse",
    "PluginGrantAccessRequest",
    "PluginGrantAccessResponse",
    "PluginRevokeAccessRequest",
    "PluginRevokeAccessResponse",
    "PluginValidateRequest",
    "PluginValidateResponse",
    "ResourceDeprovisionRequest",
    "ResourceDeprovisionResponse",
    "ResourceGetAccessRequest",
    "ResourceGetAccessResponse",
    "ResourceGrantAccessRequest",
    "ResourceGrantAccessResponse",
    "ResourceListRequest",
    "ResourceListResponse",
    "ResourceProvisionRequest",
    "ResourceProvisionResponse",
    "ResourceRevokeAccessRequest",
    "ResourceRevokeAccessResponse",
    "StatusCode",
    "create_plugin",
    "plain",
]
