package descriptor

import (
	"context"
	"testing"

	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/conf"
)

func TestDescribeCapabilities(t *testing.T) {
	source := NewSource(Options{
		ServiceID: "sandbox.local.test",
		Version:   "test",
		HTTPAddr:  "sandbox-service:8081",
		GRPCAddr:  "sandbox-service:9081",
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
	if len(descriptor.GetSandboxProfiles()) != 2 {
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
	if !hasFeature(control, "create_hosted_workspace") || !hasFeature(control, "get_hosted_workspace") {
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
	if resource.GetStatus() != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL || !hasFeature(resource, "export_workspace_path") {
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
	source := NewSourceFromConfig(cfg, "test")
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
