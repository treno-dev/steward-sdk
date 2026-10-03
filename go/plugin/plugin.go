// Package plugin is the plugin part of the Steward SDK for Go: the plumbing between the plugin
// contract (gRPC, see ../../proto) and the functions a plugin writes.
//
//	integration := plugin.New(plugin.Options{Name: "github", Version: "0.1.0", Inputs: inputs, Validate: validate})
//	integration.Resource("repository", plugin.ResourceOptions{Provision: provision, Deprovision: deprovision})
//	integration.Serve()
//
// A plugin is one integration, like a provider. It declares that integration and each of its resource
// or application kinds once, with the handlers attached. The SDK routes every call to the right
// handler by kind, builds the description Steward asks for (including each kind's capabilities, which
// follow from the handlers it has), serves the gRPC services, prints the handshake line, answers the
// health check, and answers any call without a handler with UNIMPLEMENTED.
//
// Everything a plugin logs must go to stderr: stdout is reserved for the handshake line.
//
// Handlers receive and return the generated contract messages. Fail a call with a gRPC status, for
// example status.Error(codes.FailedPrecondition, "..."); any other error is reported as INTERNAL.
package plugin

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pluginv1 "github.com/treno-dev/steward-sdk/go/gen/steward/plugin/v1"
)

// Options declare the plugin and its integration: what is needed to create an integration from it,
// and, when the integration supports access as a whole, the handlers for that.
type Options struct {
	Name        string
	Version     string
	Title       string
	Description string

	// Inputs is what is required to create an integration, credentials included.
	Inputs []*InputDefinition

	// Validate checks the inputs and credentials.
	Validate func(context.Context, *IntegrationValidateRequest) (*IntegrationValidateResponse, error)

	// What can be granted on the integration as a whole, and the handlers that grant it.
	Permissions []*PermissionDefinition
	Roles       []*RoleDefinition

	GrantAccess  func(context.Context, *IntegrationGrantAccessRequest) (*IntegrationGrantAccessResponse, error)
	RevokeAccess func(context.Context, *IntegrationRevokeAccessRequest) (*IntegrationRevokeAccessResponse, error)
	GetAccess    func(context.Context, *IntegrationGetAccessRequest) (*IntegrationGetAccessResponse, error)
}

// ResourceOptions describe a kind of resource the integration manages, with its handlers. A call
// for something without a handler fails with UNIMPLEMENTED.
type ResourceOptions struct {
	Title       string
	Description string

	// Inputs is what a resource of this kind takes besides its name; Outputs is what it produces.
	Inputs  []*InputDefinition
	Outputs []*OutputDefinition

	// What can be granted on this kind: the tool's roles, the permissions, or both.
	Permissions []*PermissionDefinition
	Roles       []*RoleDefinition

	Provision   func(context.Context, *ResourceProvisionRequest) (*ResourceProvisionResponse, error)
	Deprovision func(context.Context, *ResourceDeprovisionRequest) (*ResourceDeprovisionResponse, error)
	List        func(context.Context, *ResourceListRequest) (*ResourceListResponse, error)

	GrantAccess  func(context.Context, *ResourceGrantAccessRequest) (*ResourceGrantAccessResponse, error)
	RevokeAccess func(context.Context, *ResourceRevokeAccessRequest) (*ResourceRevokeAccessResponse, error)
	GetAccess    func(context.Context, *ResourceGetAccessRequest) (*ResourceGetAccessResponse, error)
}

// ApplicationOptions describe a kind of application the integration runs, with its handlers.
type ApplicationOptions struct {
	Title       string
	Description string

	Inputs  []*InputDefinition
	Outputs []*OutputDefinition

	// Sources is what it can be deployed from: "git" or "image".
	Sources []string

	Create       func(context.Context, *ApplicationCreateRequest) (*ApplicationCreateResponse, error)
	Delete       func(context.Context, *ApplicationDeleteRequest) (*ApplicationDeleteResponse, error)
	Deploy       func(context.Context, *ApplicationDeployRequest) (*ApplicationDeployResponse, error)
	SetVariables func(context.Context, *ApplicationSetVariablesRequest) (*ApplicationSetVariablesResponse, error)
	List         func(context.Context, *ApplicationListRequest) (*ApplicationListResponse, error)
}

// Plugin is one integration. Declare its resources and applications, then call Serve.
type Plugin struct {
	options      Options
	resources    map[string]ResourceOptions
	applications map[string]ApplicationOptions

	// Declaration order, so the description lists kinds the way the plugin declared them.
	resourceOrder    []string
	applicationOrder []string
}

// New creates a plugin, which is one integration.
func New(options Options) *Plugin {
	return &Plugin{
		options:      options,
		resources:    map[string]ResourceOptions{},
		applications: map[string]ApplicationOptions{},
	}
}

// Resource declares a kind of resource the integration manages. The kind is unique within the
// plugin, such as "repository".
func (p *Plugin) Resource(kind string, options ResourceOptions) *Plugin {
	if _, declared := p.resources[kind]; !declared {
		p.resourceOrder = append(p.resourceOrder, kind)
	}

	p.resources[kind] = options

	return p
}

// Application declares a kind of application the integration runs. The kind is unique within the
// plugin, such as "laravel".
func (p *Plugin) Application(kind string, options ApplicationOptions) *Plugin {
	if _, declared := p.applications[kind]; !declared {
		p.applicationOrder = append(p.applicationOrder, kind)
	}

	p.applications[kind] = options

	return p
}

func (p *Plugin) hasIntegrationAccess() bool {
	o := p.options

	return o.GrantAccess != nil || o.RevokeAccess != nil || o.GetAccess != nil
}

func (p *Plugin) describe() *pluginv1.DescribeResponse {
	o := p.options
	integration := &pluginv1.IntegrationDefinition{
		Title:       o.Title,
		Description: o.Description,
		Inputs:      o.Inputs,
		Permissions: o.Permissions,
		Roles:       o.Roles,
	}

	if p.hasIntegrationAccess() {
		integration.Capabilities = []pluginv1.IntegrationCapability{pluginv1.IntegrationCapability_INTEGRATION_CAPABILITY_ACCESS}
	}

	for _, name := range p.resourceOrder {
		integration.Resources = append(integration.Resources, resourceDefinition(name, p.resources[name]))
	}

	for _, name := range p.applicationOrder {
		options := p.applications[name]
		integration.Applications = append(integration.Applications, &pluginv1.ApplicationDefinition{
			Kind:        name,
			Title:       options.Title,
			Description: options.Description,
			Inputs:      options.Inputs,
			Sources:     options.Sources,
			Outputs:     options.Outputs,
		})
	}

	return &pluginv1.DescribeResponse{Name: o.Name, Version: o.Version, Integration: integration}
}

func resourceDefinition(kind string, options ResourceOptions) *pluginv1.ResourceDefinition {
	var capabilities []pluginv1.ResourceCapability

	if options.GrantAccess != nil || options.RevokeAccess != nil || options.GetAccess != nil {
		capabilities = append(capabilities, pluginv1.ResourceCapability_RESOURCE_CAPABILITY_ACCESS)
	}

	if options.Provision != nil || options.Deprovision != nil {
		capabilities = append(capabilities, pluginv1.ResourceCapability_RESOURCE_CAPABILITY_PROVISIONING)
	}

	if options.List != nil {
		capabilities = append(capabilities, pluginv1.ResourceCapability_RESOURCE_CAPABILITY_DISCOVERY)
	}

	return &pluginv1.ResourceDefinition{
		Kind:         kind,
		Title:        options.Title,
		Description:  options.Description,
		Inputs:       options.Inputs,
		Capabilities: capabilities,
		Permissions:  options.Permissions,
		Roles:        options.Roles,
		Outputs:      options.Outputs,
	}
}

func (p *Plugin) resource(kind string) (ResourceOptions, error) {
	found, ok := p.resources[kind]
	if !ok {
		return found, status.Error(codes.NotFound, fmt.Sprintf("this plugin has no resource %q", kind))
	}

	return found, nil
}

func (p *Plugin) application(kind string) (ApplicationOptions, error) {
	found, ok := p.applications[kind]
	if !ok {
		return found, status.Error(codes.NotFound, fmt.Sprintf("this plugin has no application %q", kind))
	}

	return found, nil
}

func unimplemented(call, target string) error {
	return status.Error(codes.Unimplemented, fmt.Sprintf("%s is not implemented for this %s", call, target))
}
