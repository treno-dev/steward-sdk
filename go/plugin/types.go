package plugin

import (
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"

	pluginv1 "github.com/treno-dev/steward-sdk/go/gen/steward/plugin/v1"
)

// Struct turns a map into the Struct the contract uses for open-ended values, such as a response's
// Outputs or an identity's Attrs. Values must be what JSON can hold (strings, numbers, booleans, nil,
// and maps and slices of those). Anything else is a mistake in the plugin, so it panics, and the SDK
// reports a panic in a handler as an INTERNAL error.
func Struct(values map[string]any) *structpb.Struct {
	result, err := structpb.NewStruct(values)
	if err != nil {
		panic(fmt.Sprintf("plugin.Struct: %v", err))
	}

	return result
}

// Value turns a Go value into the Value the contract uses for an input's Default. Same rules as
// Struct.
func Value(value any) *structpb.Value {
	result, err := structpb.NewValue(value)
	if err != nil {
		panic(fmt.Sprintf("plugin.Value: %v", err))
	}

	return result
}

// The messages a plugin works with, from the generated contract, so a plugin imports one package.
// Requests and responses are named for what they handle (the entity, then the call).
type (
	PluginConfig      = pluginv1.PluginConfig
	Resource          = pluginv1.Resource
	Application       = pluginv1.Application
	ApplicationSource = pluginv1.ApplicationSource
	Variable          = pluginv1.Variable
	Identity          = pluginv1.Identity
	Role              = pluginv1.Role
	Permission        = pluginv1.Permission

	InputDefinition      = pluginv1.InputDefinition
	SecretDefinition     = pluginv1.SecretDefinition
	OutputDefinition     = pluginv1.OutputDefinition
	PermissionDefinition = pluginv1.PermissionDefinition
	RoleDefinition       = pluginv1.RoleDefinition
	ValidationError      = pluginv1.ValidationError

	PluginValidateRequest      = pluginv1.ValidateRequest
	PluginValidateResponse     = pluginv1.ValidateResponse
	PluginGrantAccessRequest   = pluginv1.PluginServiceGrantAccessRequest
	PluginGrantAccessResponse  = pluginv1.PluginServiceGrantAccessResponse
	PluginRevokeAccessRequest  = pluginv1.PluginServiceRevokeAccessRequest
	PluginRevokeAccessResponse = pluginv1.PluginServiceRevokeAccessResponse
	PluginGetAccessRequest     = pluginv1.PluginServiceGetAccessRequest
	PluginGetAccessResponse    = pluginv1.PluginServiceGetAccessResponse

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

	ApplicationCreateRequest   = pluginv1.ApplicationServiceCreateRequest
	ApplicationCreateResponse  = pluginv1.ApplicationServiceCreateResponse
	ApplicationDestroyRequest  = pluginv1.ApplicationServiceDestroyRequest
	ApplicationDestroyResponse = pluginv1.ApplicationServiceDestroyResponse
	ApplicationDeployRequest   = pluginv1.ApplicationServiceDeployRequest
	ApplicationDeployResponse  = pluginv1.ApplicationServiceDeployResponse
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
