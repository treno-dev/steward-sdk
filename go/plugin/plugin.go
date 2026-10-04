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

	// Inputs are the settings the plugin is configured with; Secrets are its credentials.
	Inputs  []*InputDefinition
	Secrets []*SecretDefinition

	// Validate checks the inputs and credentials.
	Validate func(context.Context, *PluginValidateRequest) (*PluginValidateResponse, error)

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

// Plugin is what a plugin author builds. Declare its resources and applications, then call Serve.
type Plugin struct {
	options      Options
	resources    map[string]ResourceOptions
	applications map[string]ApplicationOptions

	// Declaration order, so the description lists kinds the way the plugin declared them.
	resourceOrder    []string
	applicationOrder []string
}

// New creates a plugin.
func New(options Options) *Plugin {
	return &Plugin{
		options:      options,
		resources:    map[string]ResourceOptions{},
		applications: map[string]ApplicationOptions{},
	}
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

func (p *Plugin) describe() *pluginv1.DescribeResponse {
	o := p.options
	definition := &pluginv1.PluginDefinition{
		Title:       o.Title,
		Description: o.Description,
		Inputs:      o.Inputs,
		Secrets:     o.Secrets,
		Permissions: o.Permissions,
		Roles:       o.Roles,
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

	return &pluginv1.DescribeResponse{Name: o.Name, Version: o.Version, Definition: definition}
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

func unimplemented(call, target string) error {
	return status.Error(codes.Unimplemented, fmt.Sprintf("%s is not implemented for this %s", call, target))
}
