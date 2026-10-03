"""The Steward plugin SDK for Python: the plumbing between the plugin contract (gRPC, see
../../proto) and the functions a plugin writes.

    plugin = create_plugin(name="github", version="0.1.0", inputs=[...], validate=validate)
    plugin.resource(kind="repository", inputs=[...], provision=provision, deprovision=deprovision)
    plugin.serve()

A plugin is one integration, like a provider. It declares that integration and each of its resource
or application kinds once, with the handlers attached. The SDK routes every call to the right
handler by kind, builds the description Steward asks for (including each kind's capabilities, which
follow from the handlers it has), serves the gRPC services, prints the handshake line, answers the
health check, and answers any call without a handler with UNIMPLEMENTED.

Everything a plugin logs must go to stderr: stdout is reserved for the handshake line.
"""

from steward_plugin._gen.steward.plugin.v1.plugin_pb2 import *  # noqa: F401,F403
from steward_plugin._plugin import (
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
    IntegrationGetAccessRequest,
    IntegrationGetAccessResponse,
    IntegrationGrantAccessRequest,
    IntegrationGrantAccessResponse,
    IntegrationRevokeAccessRequest,
    IntegrationRevokeAccessResponse,
    IntegrationValidateRequest,
    IntegrationValidateResponse,
    Plugin,
    PluginError,
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
    "IntegrationGetAccessRequest",
    "IntegrationGetAccessResponse",
    "IntegrationGrantAccessRequest",
    "IntegrationGrantAccessResponse",
    "IntegrationRevokeAccessRequest",
    "IntegrationRevokeAccessResponse",
    "IntegrationValidateRequest",
    "IntegrationValidateResponse",
    "Plugin",
    "PluginError",
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
