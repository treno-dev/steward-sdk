package steward

import pluginv1 "github.com/treno-dev/steward/sdk/plugin/go/gen/steward/plugin/v1"

// The messages a plugin works with, from the generated contract, so a plugin imports one package.
// Requests and responses are named for what they handle (the entity, then the call).
type (
	Integration       = pluginv1.Integration
	Resource          = pluginv1.Resource
	Application       = pluginv1.Application
	ApplicationSource = pluginv1.ApplicationSource
	Variable          = pluginv1.Variable
	Identity          = pluginv1.Identity
	Role              = pluginv1.Role
	Permission        = pluginv1.Permission

	InputDefinition      = pluginv1.InputDefinition
	OutputDefinition     = pluginv1.OutputDefinition
	PermissionDefinition = pluginv1.PermissionDefinition
	RoleDefinition       = pluginv1.RoleDefinition
	ValidationError      = pluginv1.ValidationError

	IntegrationValidateRequest      = pluginv1.ValidateRequest
	IntegrationValidateResponse     = pluginv1.ValidateResponse
	IntegrationGrantAccessRequest   = pluginv1.IntegrationServiceGrantAccessRequest
	IntegrationGrantAccessResponse  = pluginv1.IntegrationServiceGrantAccessResponse
	IntegrationRevokeAccessRequest  = pluginv1.IntegrationServiceRevokeAccessRequest
	IntegrationRevokeAccessResponse = pluginv1.IntegrationServiceRevokeAccessResponse
	IntegrationGetAccessRequest     = pluginv1.IntegrationServiceGetAccessRequest
	IntegrationGetAccessResponse    = pluginv1.IntegrationServiceGetAccessResponse

	ResourceProvisionRequest     = pluginv1.ResourceServiceProvisionRequest
	ResourceProvisionResponse    = pluginv1.ResourceServiceProvisionResponse
	ResourceDeprovisionRequest   = pluginv1.ResourceServiceDeprovisionRequest
	ResourceDeprovisionResponse  = pluginv1.ResourceServiceDeprovisionResponse
	ResourceListRequest          = pluginv1.ResourceServiceListRequest
	ResourceListResponse         = pluginv1.ResourceServiceListResponse
	ResourceGrantAccessRequest   = pluginv1.ResourceServiceGrantAccessRequest
	ResourceGrantAccessResponse  = pluginv1.ResourceServiceGrantAccessResponse
	ResourceRevokeAccessRequest  = pluginv1.ResourceServiceRevokeAccessRequest
	ResourceRevokeAccessResponse = pluginv1.ResourceServiceRevokeAccessResponse
	ResourceGetAccessRequest     = pluginv1.ResourceServiceGetAccessRequest
	ResourceGetAccessResponse    = pluginv1.ResourceServiceGetAccessResponse

	ApplicationCreateRequest         = pluginv1.ApplicationServiceCreateRequest
	ApplicationCreateResponse        = pluginv1.ApplicationServiceCreateResponse
	ApplicationDeleteRequest         = pluginv1.ApplicationServiceDeleteRequest
	ApplicationDeleteResponse        = pluginv1.ApplicationServiceDeleteResponse
	ApplicationDeployRequest         = pluginv1.ApplicationServiceDeployRequest
	ApplicationDeployResponse        = pluginv1.ApplicationServiceDeployResponse
	ApplicationSetVariablesRequest   = pluginv1.ApplicationServiceSetVariablesRequest
	ApplicationSetVariablesResponse  = pluginv1.ApplicationServiceSetVariablesResponse
	ApplicationListRequest           = pluginv1.ApplicationServiceListRequest
	ApplicationListResponse          = pluginv1.ApplicationServiceListResponse
)

// The types an InputDefinition or OutputDefinition can have.
const (
	TypeString  = "string"
	TypeNumber  = "number"
	TypeBoolean = "boolean"
	TypeSelect  = "select"
	TypeList    = "list"
	TypeMap     = "map"
)
