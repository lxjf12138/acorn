package descriptor

import (
	"context"
	"testing"

	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
)

func TestDescribeCapabilities(t *testing.T) {
	source := NewSource(Options{
		ServiceID:   "sandbox.local.test",
		Version:     "test",
		HTTPAddr:    "127.0.0.1:8081",
		GRPCAddr:    "127.0.0.1:9081",
		MCPEndpoint: "/mcp",
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
	if status := surfaceStatus(descriptor, "resource"); status != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED {
		t.Fatalf("unexpected resource surface status: %s", status)
	}
	if endpoint := endpointByName(descriptor, "control-http"); endpoint.GetAddress() != "127.0.0.1:8081" || endpoint.GetStatus() != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_IMPLEMENTED {
		t.Fatalf("unexpected control HTTP endpoint: %+v", endpoint)
	}
	if endpoint := endpointByName(descriptor, "control-grpc"); endpoint.GetAddress() != "127.0.0.1:9081" || endpoint.GetStatus() != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_IMPLEMENTED {
		t.Fatalf("unexpected control gRPC endpoint: %+v", endpoint)
	}
}

func surfaceStatus(descriptor *capabilityv1.CapabilityDescriptor, name string) capabilityv1.ImplementationStatus {
	for _, surface := range descriptor.GetSurfaces() {
		if surface.GetName() == name {
			return surface.GetStatus()
		}
	}
	return capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_UNSPECIFIED
}

func endpointByName(descriptor *capabilityv1.CapabilityDescriptor, name string) *capabilityv1.EndpointDescriptor {
	for _, endpoint := range descriptor.GetEndpoints() {
		if endpoint.GetName() == name {
			return endpoint
		}
	}
	return nil
}
