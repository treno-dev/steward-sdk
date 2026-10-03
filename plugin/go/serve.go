package steward

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	pluginv1 "github.com/treno-dev/steward/sdk/plugin/go/gen/steward/plugin/v1"
)

// The version of the Steward plugin contract this plugin speaks.
const appProtocolVersion = 1

// Serve starts serving the plugin and blocks until the runner stops it. It prints the handshake line
// the runner reads, so nothing else may be written to standard output. A plugin that cannot start
// exits with status 1.
func (p *Plugin) Serve() {
	if err := p.serve(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (p *Plugin) serve() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	server := grpc.NewServer()

	pluginv1.RegisterPluginServiceServer(server, pluginService{plugin: p})

	if p.hasIntegrationAccess() {
		pluginv1.RegisterIntegrationServiceServer(server, integrationService{plugin: p})
	}

	if len(p.resources) > 0 {
		pluginv1.RegisterResourceServiceServer(server, resourceService{plugin: p})
	}

	if len(p.applications) > 0 {
		pluginv1.RegisterApplicationServiceServer(server, applicationService{plugin: p})
	}

	status := health.NewServer()
	status.SetServingStatus("plugin", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, status)

	// CORE-PROTOCOL-VERSION | APP-PROTOCOL-VERSION | NETWORK-TYPE | NETWORK-ADDR | PROTOCOL
	fmt.Printf("1|%d|tcp|%s|grpc\n", appProtocolVersion, listener.Addr())

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signals
		server.GracefulStop()
	}()

	return server.Serve(listener)
}

// The services route each call to the handler of the same name on the integration or on the kind the
// request is about, and answer a call without a handler with UNIMPLEMENTED. The generated Unimplemented
// servers keep the plugin compatible when the contract gains calls.

type pluginService struct {
	pluginv1.UnimplementedPluginServiceServer
	plugin *Plugin
}

func (s pluginService) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	return s.plugin.describe(), nil
}

func (s pluginService) Validate(ctx context.Context, request *IntegrationValidateRequest) (*IntegrationValidateResponse, error) {
	if s.plugin.options.Validate == nil {
		return nil, unimplemented("validate", "integration")
	}

	return s.plugin.options.Validate(ctx, request)
}

type integrationService struct {
	pluginv1.UnimplementedIntegrationServiceServer
	plugin *Plugin
}

func (s integrationService) GrantAccess(ctx context.Context, request *IntegrationGrantAccessRequest) (*IntegrationGrantAccessResponse, error) {
	if s.plugin.options.GrantAccess == nil {
		return nil, unimplemented("grant_access", "integration")
	}

	return s.plugin.options.GrantAccess(ctx, request)
}

func (s integrationService) RevokeAccess(ctx context.Context, request *IntegrationRevokeAccessRequest) (*IntegrationRevokeAccessResponse, error) {
	if s.plugin.options.RevokeAccess == nil {
		return nil, unimplemented("revoke_access", "integration")
	}

	return s.plugin.options.RevokeAccess(ctx, request)
}

func (s integrationService) GetAccess(ctx context.Context, request *IntegrationGetAccessRequest) (*IntegrationGetAccessResponse, error) {
	if s.plugin.options.GetAccess == nil {
		return nil, unimplemented("get_access", "integration")
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

func (s applicationService) Delete(ctx context.Context, request *ApplicationDeleteRequest) (*ApplicationDeleteResponse, error) {
	kind, err := s.plugin.application(request.GetApplication().GetKind())
	if err != nil {
		return nil, err
	}

	if kind.Delete == nil {
		return nil, unimplemented("delete", "application")
	}

	return kind.Delete(ctx, request)
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

func (s applicationService) SetVariables(ctx context.Context, request *ApplicationSetVariablesRequest) (*ApplicationSetVariablesResponse, error) {
	kind, err := s.plugin.application(request.GetApplication().GetKind())
	if err != nil {
		return nil, err
	}

	if kind.SetVariables == nil {
		return nil, unimplemented("set_variables", "application")
	}

	return kind.SetVariables(ctx, request)
}

func (s applicationService) List(ctx context.Context, request *ApplicationListRequest) (*ApplicationListResponse, error) {
	kind, err := s.plugin.application(request.GetKind())
	if err != nil {
		return nil, err
	}

	if kind.List == nil {
		return nil, unimplemented("list", "application")
	}

	return kind.List(ctx, request)
}
