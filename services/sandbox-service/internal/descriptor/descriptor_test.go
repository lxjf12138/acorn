package descriptor

import (
	"context"
	"testing"

	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/conf"
	profiledomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/profile"
)

func TestDescribeCapabilities(t *testing.T) {
	source := NewSource(Options{
		ServiceID: "sandbox.local.test",
		Version:   "test",
		HTTPAddr:  "sandbox-service:8081",
		GRPCAddr:  "sandbox-service:9081",
		ProfileRegistry: profiledomain.NewMemoryRegistry([]*profiledomain.Profile{
			localProcessProfile(),
		}),
	})
	descriptor, err := source.DescribeCapabilities(context.Background())
	if err != nil {
		t.Fatalf("DescribeCapabilities returned error: %v", err)
	}
	if descriptor.GetServiceId() != "sandbox.local.test" {
		t.Fatalf("unexpected service id: %q", descriptor.GetServiceId())
	}
	if descriptor.GetVersion() != "test" {
		t.Fatalf("unexpected version: %q", descriptor.GetVersion())
	}
	if descriptor.GetContract() != "acorn.sandbox" {
		t.Fatalf("unexpected contract: %q", descriptor.GetContract())
	}
	if len(descriptor.GetSandboxProfiles()) != 1 {
		t.Fatalf("unexpected profiles: %v", descriptor.GetSandboxProfiles())
	}
	if descriptor.GetAgentSurface().GetStatus() != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED {
		t.Fatalf("unexpected agent surface status: %s", descriptor.GetAgentSurface().GetStatus())
	}
	if mcp := descriptor.GetAgentSurface().GetMcp()[0]; mcp.GetStatus() != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED || mcp.GetEndpoint() != "/mcp" {
		t.Fatalf("unexpected MCP agent surface: %+v", mcp)
	}
	if status := surfaceStatus(descriptor, "control"); status != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL {
		t.Fatalf("unexpected control surface status: %s", status)
	}
	control := surfaceByName(descriptor, "control")
	if !hasFeature(control, "create_hosted_workspace") || !hasFeature(control, "get_hosted_workspace") || !hasFeature(control, "exec_workspace_command") {
		t.Fatalf("control surface missing hosted workspace features: %+v", control.GetFeatures())
	}
	state := surfaceByName(descriptor, "state")
	if state.GetStatus() != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL || !hasFeature(state, "workspace_state") {
		t.Fatalf("unexpected state surface: %+v", state)
	}
	view := surfaceByName(descriptor, "view")
	if view.GetStatus() != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL || !hasFeature(view, "list_workspace_dir") || !hasFeature(view, "preview_workspace_file") {
		t.Fatalf("unexpected view surface: %+v", view)
	}
	resource := surfaceByName(descriptor, "resource")
	if resource.GetStatus() != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL ||
		!hasFeature(resource, "export_workspace_path") ||
		!hasFeature(resource, "open_resource") ||
		!hasFeature(resource, "import_resource") {
		t.Fatalf("unexpected resource surface: %+v", resource)
	}
	if endpoint := endpointByName(descriptor, "control-http"); endpoint.GetAddress() != "sandbox-service:8081" || endpoint.GetStatus() != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_IMPLEMENTED {
		t.Fatalf("unexpected control HTTP endpoint: %+v", endpoint)
	}
	if endpoint := endpointByName(descriptor, "control-grpc"); endpoint.GetAddress() != "sandbox-service:9081" || endpoint.GetStatus() != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_IMPLEMENTED {
		t.Fatalf("unexpected control gRPC endpoint: %+v", endpoint)
	}
	if endpoint := endpointByName(descriptor, "workspace-host-grpc"); endpoint.GetAddress() != "sandbox-service:9081" || endpoint.GetPath() != "/acorn.sandbox.v1.WorkspaceHostService" || endpoint.GetStatus() != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL {
		t.Fatalf("unexpected workspace host gRPC endpoint: %+v", endpoint)
	}
	if endpoint := endpointByName(descriptor, "workspace-view-grpc"); endpoint.GetAddress() != "sandbox-service:9081" || endpoint.GetPath() != "/acorn.sandbox.v1.WorkspaceViewService" || endpoint.GetStatus() != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL {
		t.Fatalf("unexpected workspace view gRPC endpoint: %+v", endpoint)
	}
	if endpoint := endpointByName(descriptor, "workspace-transfer-grpc"); endpoint.GetAddress() != "sandbox-service:9081" || endpoint.GetPath() != "/acorn.sandbox.v1.WorkspaceTransferService" || endpoint.GetStatus() != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL {
		t.Fatalf("unexpected workspace transfer gRPC endpoint: %+v", endpoint)
	}
	if endpoint := endpointByName(descriptor, "workspace-exec-grpc"); endpoint.GetAddress() != "sandbox-service:9081" || endpoint.GetPath() != "/acorn.sandbox.v1.WorkspaceExecService" || endpoint.GetStatus() != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL {
		t.Fatalf("unexpected workspace exec gRPC endpoint: %+v", endpoint)
	}
	if endpoint := endpointByName(descriptor, "resource-content-grpc"); endpoint.GetAddress() != "sandbox-service:9081" || endpoint.GetPath() != "/acorn.resource.v1.ResourceContentService" || endpoint.GetStatus() != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL {
		t.Fatalf("unexpected resource content gRPC endpoint: %+v", endpoint)
	}
}

func TestDescribeCapabilitiesWithoutLocalProcess(t *testing.T) {
	source := NewSource(Options{
		ServiceID:       "sandbox.local.test",
		Version:         "test",
		HTTPAddr:        "sandbox-service:8081",
		GRPCAddr:        "sandbox-service:9081",
		ProfileRegistry: profiledomain.NewMemoryRegistry(nil),
	})
	descriptor, err := source.DescribeCapabilities(context.Background())
	if err != nil {
		t.Fatalf("DescribeCapabilities returned error: %v", err)
	}
	control := surfaceByName(descriptor, "control")
	if hasFeature(control, "exec_workspace_command") {
		t.Fatalf("control surface unexpectedly advertises exec: %+v", control.GetFeatures())
	}
	if endpoint := endpointByName(descriptor, "workspace-exec-grpc"); endpoint != nil {
		t.Fatalf("unexpected workspace exec endpoint: %+v", endpoint)
	}
	for _, profile := range descriptor.GetSandboxProfiles() {
		if profile.GetId() == "local-process-dev" {
			t.Fatalf("unexpected local-process-dev profile while disabled: %+v", profile)
		}
	}
}

func TestNewSourceFromConfigUsesServiceIDAndNameSeparately(t *testing.T) {
	cfg := &conf.Config{
		Service: conf.Service{
			ID:   "sandbox-service-id",
			Name: "Sandbox Display Name",
		},
		Server: conf.Server{
			HTTP: conf.HTTP{AdvertiseAddr: "sandbox-service:8081"},
			GRPC: conf.GRPC{AdvertiseAddr: "sandbox-service:9081"},
		},
	}
	source := NewSourceFromConfig(cfg, "test", profiledomain.NewMemoryRegistry(nil))
	descriptor, err := source.DescribeCapabilities(context.Background())
	if err != nil {
		t.Fatalf("DescribeCapabilities returned error: %v", err)
	}
	if descriptor.GetServiceId() != "sandbox-service-id" {
		t.Fatalf("unexpected service id: %q", descriptor.GetServiceId())
	}
	if descriptor.GetDisplayName() != "Sandbox Display Name" {
		t.Fatalf("unexpected display name: %q", descriptor.GetDisplayName())
	}
}

func localProcessProfile() *profiledomain.Profile {
	return &profiledomain.Profile{
		ID:             profiledomain.LocalProcessDevID,
		DisplayName:    "Local Process Dev Backend",
		Enabled:        true,
		Default:        true,
		IsolationClass: profiledomain.IsolationDevProcess,
		BackendID:      profiledomain.LocalProcessDevID,
		Capabilities: []profiledomain.Capability{
			profiledomain.CapabilityWorkspaceView,
			profiledomain.CapabilityWorkspaceResource,
			profiledomain.CapabilityWorkspaceExec,
		},
		Metadata: map[string]string{"dev_only": "true"},
	}
}

func surfaceStatus(descriptor *capabilityv1.CapabilityDescriptor, name string) capabilityv1.ImplementationStatus {
	return surfaceByName(descriptor, name).GetStatus()
}

func surfaceByName(descriptor *capabilityv1.CapabilityDescriptor, name string) *capabilityv1.SurfaceDescriptor {
	for _, surface := range descriptor.GetSurfaces() {
		if surface.GetName() == name {
			return surface
		}
	}
	return nil
}

func endpointByName(descriptor *capabilityv1.CapabilityDescriptor, name string) *capabilityv1.EndpointDescriptor {
	for _, endpoint := range descriptor.GetEndpoints() {
		if endpoint.GetName() == name {
			return endpoint
		}
	}
	return nil
}

func hasFeature(surface *capabilityv1.SurfaceDescriptor, feature string) bool {
	for _, candidate := range surface.GetFeatures() {
		if candidate == feature {
			return true
		}
	}
	return false
}
