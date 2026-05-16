package descriptor

import (
	"context"
	"testing"

	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
)

func TestDescribeCapabilities(t *testing.T) {
	source := NewSource()
	descriptor, err := source.DescribeCapabilities(context.Background())
	if err != nil {
		t.Fatalf("DescribeCapabilities returned error: %v", err)
	}
	if descriptor.GetServiceId() != ServiceID {
		t.Fatalf("unexpected service id: %q", descriptor.GetServiceId())
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
	if status := surfaceStatus(descriptor, "control"); status != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_IMPLEMENTED {
		t.Fatalf("unexpected control surface status: %s", status)
	}
	if status := surfaceStatus(descriptor, "resource"); status != capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED {
		t.Fatalf("unexpected resource surface status: %s", status)
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
