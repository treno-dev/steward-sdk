// Package plugin is the plugin part of the Steward SDK for Go: the plumbing between the plugin
// contract (gRPC, see ../../proto) and the functions a plugin writes.
//
//	p := plugin.New(plugin.Options{Name: "github", Version: "0.1.0", Inputs: inputs, Validate: validate})
//	p.Resource("repository", plugin.ResourceOptions{Provision: provision, Deprovision: deprovision})
//	p.Serve()
//
// A plugin declares itself and each of its resource or application kinds once, with the handlers
// attached. The SDK routes every call to the right
// handler by kind, builds the description Steward asks for, serves the gRPC services, prints the
// handshake line, answers the health check, and answers any call without a handler with UNIMPLEMENTED.
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

// Options declare the plugin: what is needed to configure it and, when it supports access as a
// whole, the handlers for that.
type Options struct {
	Name        string
	Version     string
	Title       string
	Description string

	// IdentityAudience is the audience of the token that proves who an integration is, such as
	// "sts.amazonaws.com". A plugin that declares it is sent the token, signed by Steward, in the
	// identity token of the integration of each call.
	IdentityAudience string

	// Inputs are the settings the plugin is configured with; Secrets are its credentials.
	Inputs  []*InputDefinition
	Secrets []*SecretDefinition

	// Validate checks the inputs and credentials.
	Validate func(context.Context, *PluginValidateRequest) (*PluginValidateResponse, error)

	// SetupLinks says what the person does in another tool before the settings of an integration,
	// such as a page that creates a role, as links with the address already filled in. It is given
	// the integration when a runner installs it, and nothing when the description is read for the
	// docs, so it has to answer for an integration that is nil.
	SetupLinks func(context.Context, *Integration) []*SetupLink

	// What can be granted on the plugin as a whole, and the handlers that grant it.
	Permissions []*PermissionDefinition
	Roles       []*RoleDefinition

	GrantAccess  func(context.Context, *PluginGrantAccessRequest) (*PluginGrantAccessResponse, error)
	RevokeAccess func(context.Context, *PluginRevokeAccessRequest) (*PluginRevokeAccessResponse, error)
	GetAccess    func(context.Context, *PluginGetAccessRequest) (*PluginGetAccessResponse, error)
}

// ResourceOptions describe a kind of resource the plugin manages, with its handlers. A call
// for something without a handler fails with UNIMPLEMENTED.
type ResourceOptions struct {
	Title       string
	Description string

	// Inputs is what a resource of this kind takes besides its name, Secrets its credentials, and
	// Outputs what it produces.
	Inputs  []*InputDefinition
	Secrets []*SecretDefinition
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

// ApplicationOptions describe a kind of application the plugin runs, with its handlers.
type ApplicationOptions struct {
	Title       string
	Description string

	Inputs  []*InputDefinition
	Secrets []*SecretDefinition
	Outputs []*OutputDefinition

	// Sources are the source types it can be deployed from: "github", "registry", "s3" or "raw".
	Sources []string

	Create  func(context.Context, *ApplicationCreateRequest) (*ApplicationCreateResponse, error)
	Destroy func(context.Context, *ApplicationDestroyRequest) (*ApplicationDestroyResponse, error)
	Deploy  func(context.Context, *ApplicationDeployRequest) (*ApplicationDeployResponse, error)
}

// SecretEngineOptions describe a place the plugin can keep secrets, with its handlers. A call for
// something without a handler fails with UNIMPLEMENTED. A value is never shown to a person: Get is
// how the runner resolves a secret for a call.
type SecretEngineOptions struct {
	Title       string
	Description string

	// Icon is the name of an icon for the engine, such as "database" or "vault".
	Icon string

	// Inputs are the settings of a store made with this engine, such as a prefix. The credentials
	// are the integration's.
	Inputs []*InputDefinition

	Put    func(context.Context, *SecretEnginePutRequest) (*SecretEnginePutResponse, error)
	Delete func(context.Context, *SecretEngineDeleteRequest) (*SecretEngineDeleteResponse, error)
	Get    func(context.Context, *SecretEngineGetRequest) (*SecretEngineGetResponse, error)
}

// Plugin is what a plugin author builds. Declare its resources, applications and secret engines,
// then call Serve.
type Plugin struct {
	options       Options
	resources     map[string]ResourceOptions
	applications  map[string]ApplicationOptions
	secretEngines map[string]SecretEngineOptions

	// Declaration order, so the description lists kinds the way the plugin declared them.
	resourceOrder     []string
	applicationOrder  []string
	secretEngineOrder []string
}

// New creates a plugin.
func New(options Options) *Plugin {
	return &Plugin{
		options:       options,
		resources:     map[string]ResourceOptions{},
		applications:  map[string]ApplicationOptions{},
		secretEngines: map[string]SecretEngineOptions{},
	}
}

// SecretEngine declares a place the plugin can keep secrets. The name is unique within the plugin,
// such as "parameter-store".
func (p *Plugin) SecretEngine(name string, options SecretEngineOptions) *Plugin {
	if _, declared := p.secretEngines[name]; !declared {
		p.secretEngineOrder = append(p.secretEngineOrder, name)
	}

	p.secretEngines[name] = options

	return p
}

// Resource declares a kind of resource the plugin manages. The kind is unique within the
// plugin, such as "repository".
func (p *Plugin) Resource(kind string, options ResourceOptions) *Plugin {
	if _, declared := p.resources[kind]; !declared {
		p.resourceOrder = append(p.resourceOrder, kind)
	}

	p.resources[kind] = options

	return p
}

// Application declares a kind of application the plugin runs. The kind is unique within the
// plugin, such as "laravel".
func (p *Plugin) Application(kind string, options ApplicationOptions) *Plugin {
	if _, declared := p.applications[kind]; !declared {
		p.applicationOrder = append(p.applicationOrder, kind)
	}

	p.applications[kind] = options

	return p
}

func (p *Plugin) describe(ctx context.Context, integration *Integration) *pluginv1.DescribeResponse {
	o := p.options
	definition := &pluginv1.PluginDefinition{
		Title:       o.Title,
		Description: o.Description,
		Inputs:      o.Inputs,
		Secrets:     o.Secrets,
		Permissions: o.Permissions,
		Roles:       o.Roles,

		IdentityAudience: o.IdentityAudience,
	}

	for _, name := range p.resourceOrder {
		definition.Resources = append(definition.Resources, resourceDefinition(name, p.resources[name]))
	}

	for _, name := range p.applicationOrder {
		options := p.applications[name]
		definition.Applications = append(definition.Applications, &pluginv1.ApplicationDefinition{
			Kind:        name,
			Title:       options.Title,
			Description: options.Description,
			Inputs:      options.Inputs,
			Secrets:     options.Secrets,
			Sources:     options.Sources,
			Outputs:     options.Outputs,
		})
	}

	for _, name := range p.secretEngineOrder {
		options := p.secretEngines[name]
		definition.SecretEngines = append(definition.SecretEngines, &pluginv1.SecretEngineDefinition{
			Name:        name,
			Title:       options.Title,
			Description: options.Description,
			Icon:        options.Icon,
			Inputs:      options.Inputs,
		})
	}

	response := &pluginv1.DescribeResponse{Name: o.Name, Version: o.Version, Definition: definition}

	// The links are for an integration, so a description that was asked for without one has none.
	if integration != nil && o.SetupLinks != nil {
		response.SetupLinks = o.SetupLinks(ctx, integration)
	}

	return response
}

func resourceDefinition(kind string, options ResourceOptions) *pluginv1.ResourceDefinition {
	return &pluginv1.ResourceDefinition{
		Kind:        kind,
		Title:       options.Title,
		Description: options.Description,
		Inputs:      options.Inputs,
		Secrets:     options.Secrets,
		Permissions: options.Permissions,
		Roles:       options.Roles,
		Outputs:     options.Outputs,
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

func (p *Plugin) secretEngine(name string) (SecretEngineOptions, error) {
	found, ok := p.secretEngines[name]
	if !ok {
		return found, status.Error(codes.NotFound, fmt.Sprintf("this plugin has no secret engine %q", name))
	}

	return found, nil
}

func unimplemented(call, target string) error {
	return status.Error(codes.Unimplemented, fmt.Sprintf("%s is not implemented for this %s", call, target))
}
