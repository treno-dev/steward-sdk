package plugin

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pluginv1 "github.com/treno-dev/steward-sdk/go/gen/steward/plugin/v1"
)

// Serve starts serving the plugin and blocks until the runner stops it. It is hashicorp/go-plugin that
// does the work the runner depends on: it listens on a unix socket (TCP on Windows), prints the
// handshake line, answers the health check for the service "plugin", and shuts down when asked.
//
// A plugin must be started by a runner, which sets what Handshake describes in its environment. Started
// by hand, it says so and exits, so run it with `steward plugin validate` instead.
func (p *Plugin) Serve() {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         goplugin.PluginSet{Name: &grpcPlugin{plugin: p}},
		GRPCServer: func(options []grpc.ServerOption) *grpc.Server {
			return goplugin.DefaultGRPCServer(append(options, grpc.ChainUnaryInterceptor(recoverPanics)))
		},
	})
}

// grpcPlugin registers the services of the plugin with the gRPC server that go-plugin runs.
type grpcPlugin struct {
	goplugin.NetRPCUnsupportedPlugin
	plugin *Plugin
}

func (g *grpcPlugin) GRPCServer(_ *goplugin.GRPCBroker, server *grpc.Server) error {
	p := g.plugin

	pluginv1.RegisterPluginServiceServer(server, pluginService{plugin: p})

	if len(p.resources) > 0 {
		pluginv1.RegisterResourceServiceServer(server, resourceService{plugin: p})
	}

	if len(p.applications) > 0 {
		pluginv1.RegisterApplicationServiceServer(server, applicationService{plugin: p})
	}

	return nil
}

// GRPCClient is the runner's side. A runner uses the connection as it is, with the generated clients.
func (g *grpcPlugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, conn *grpc.ClientConn) (any, error) {
	return conn, nil
}

// ClientPlugin is what a runner passes to go-plugin, under Name, so that it gets the gRPC connection
// back from Dispense.
func ClientPlugin() goplugin.Plugin {
	return &grpcPlugin{}
}

// recoverPanics reports a panic in a handler as an INTERNAL error, with its stack on stderr, instead
// of letting it take the whole plugin down.
func recoverPanics(ctx context.Context, request any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(os.Stderr, "panic in handler: %v\n%s", recovered, debug.Stack())
			err = status.Error(codes.Internal, fmt.Sprint(recovered))
		}
	}()

	return handler(ctx, request)
}

// The services route each call to the handler of the same name on the plugin or on the kind the
// request is about, and answer a call without a handler with UNIMPLEMENTED. The generated Unimplemented
// servers keep the plugin compatible when the contract gains calls.

type pluginService struct {
	pluginv1.UnimplementedPluginServiceServer
	plugin *Plugin
}

func (s pluginService) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	return s.plugin.describe(), nil
}

func (s pluginService) Validate(ctx context.Context, request *PluginValidateRequest) (*PluginValidateResponse, error) {
	// Every plugin must answer Validate; without a handler of its own, every config is accepted.
	if s.plugin.options.Validate == nil {
		return &PluginValidateResponse{}, nil
	}

	return s.plugin.options.Validate(ctx, request)
}

func (s pluginService) GrantAccess(ctx context.Context, request *PluginGrantAccessRequest) (*PluginGrantAccessResponse, error) {
	if s.plugin.options.GrantAccess == nil {
		return nil, unimplemented("grant_access", "plugin")
	}

	return s.plugin.options.GrantAccess(ctx, request)
}

func (s pluginService) RevokeAccess(ctx context.Context, request *PluginRevokeAccessRequest) (*PluginRevokeAccessResponse, error) {
	if s.plugin.options.RevokeAccess == nil {
		return nil, unimplemented("revoke_access", "plugin")
	}

	return s.plugin.options.RevokeAccess(ctx, request)
}

func (s pluginService) GetAccess(ctx context.Context, request *PluginGetAccessRequest) (*PluginGetAccessResponse, error) {
	if s.plugin.options.GetAccess == nil {
		return nil, unimplemented("get_access", "plugin")
	}

	return s.plugin.options.GetAccess(ctx, request)
}

type resourceService struct {
	pluginv1.UnimplementedResourceServiceServer
	plugin *Plugin
}

func (s resourceService) Provision(ctx context.Context, request *ResourceProvisionRequest) (*ResourceProvisionResponse, error) {
	kind, err := s.plugin.resource(request.GetKind())
	if err != nil {
		return nil, err
	}

	if kind.Provision == nil {
		return nil, unimplemented("provision", "resource")
	}

	return kind.Provision(ctx, request)
}

func (s resourceService) Deprovision(ctx context.Context, request *ResourceDeprovisionRequest) (*ResourceDeprovisionResponse, error) {
	kind, err := s.plugin.resource(request.GetResource().GetKind())
	if err != nil {
		return nil, err
	}

	if kind.Deprovision == nil {
		return nil, unimplemented("deprovision", "resource")
	}

	return kind.Deprovision(ctx, request)
}

func (s resourceService) List(ctx context.Context, request *ResourceListRequest) (*ResourceListResponse, error) {
	kind, err := s.plugin.resource(request.GetKind())
	if err != nil {
		return nil, err
	}

	if kind.List == nil {
		return nil, unimplemented("list", "resource")
	}

	return kind.List(ctx, request)
}

func (s resourceService) GrantAccess(ctx context.Context, request *ResourceGrantAccessRequest) (*ResourceGrantAccessResponse, error) {
	kind, err := s.plugin.resource(request.GetResource().GetKind())
	if err != nil {
		return nil, err
	}

	if kind.GrantAccess == nil {
		return nil, unimplemented("grant_access", "resource")
	}

	return kind.GrantAccess(ctx, request)
}

func (s resourceService) RevokeAccess(ctx context.Context, request *ResourceRevokeAccessRequest) (*ResourceRevokeAccessResponse, error) {
	kind, err := s.plugin.resource(request.GetResource().GetKind())
	if err != nil {
		return nil, err
	}

	if kind.RevokeAccess == nil {
		return nil, unimplemented("revoke_access", "resource")
	}

	return kind.RevokeAccess(ctx, request)
}

func (s resourceService) GetAccess(ctx context.Context, request *ResourceGetAccessRequest) (*ResourceGetAccessResponse, error) {
	kind, err := s.plugin.resource(request.GetResource().GetKind())
	if err != nil {
		return nil, err
	}

	if kind.GetAccess == nil {
		return nil, unimplemented("get_access", "resource")
	}

	return kind.GetAccess(ctx, request)
}

type applicationService struct {
	pluginv1.UnimplementedApplicationServiceServer
	plugin *Plugin
}

func (s applicationService) Create(ctx context.Context, request *ApplicationCreateRequest) (*ApplicationCreateResponse, error) {
	kind, err := s.plugin.application(request.GetKind())
	if err != nil {
		return nil, err
	}

	if kind.Create == nil {
		return nil, unimplemented("create", "application")
	}

	return kind.Create(ctx, request)
}

func (s applicationService) Destroy(ctx context.Context, request *ApplicationDestroyRequest) (*ApplicationDestroyResponse, error) {
	kind, err := s.plugin.application(request.GetApplication().GetKind())
	if err != nil {
		return nil, err
	}

	if kind.Destroy == nil {
		return nil, unimplemented("destroy", "application")
	}

	return kind.Destroy(ctx, request)
}

func (s applicationService) Deploy(ctx context.Context, request *ApplicationDeployRequest) (*ApplicationDeployResponse, error) {
	kind, err := s.plugin.application(request.GetApplication().GetKind())
	if err != nil {
		return nil, err
	}

	if kind.Deploy == nil {
		return nil, unimplemented("deploy", "application")
	}

	return kind.Deploy(ctx, request)
}
