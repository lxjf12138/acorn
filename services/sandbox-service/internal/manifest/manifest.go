package manifest

import (
	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
	providerv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/provider/v1"
)

const (
	ServiceID  = "sandbox.local.dev"
	ProviderID = "sandbox"
)

// DefaultCapabilityService declares sandbox-service as the first standard
// Capability Service skeleton.
func DefaultCapabilityService() *capabilityv1.CapabilityService {
	return &capabilityv1.CapabilityService{
		Id:       ServiceID,
		Name:     "sandbox-service",
		Kind:     "sandbox",
		Contract: "acorn.sandbox",
		Version:  "dev",
		Features: []string{
			"sandboxes",
			"workspaces",
			"resource_import",
			"resource_export",
		},
	}
}

// DefaultProviderManifest declares supported surfaces without registering real
// sandbox tools. Tool definitions belong in later Agent Surface work.
func DefaultProviderManifest() *providerv1.ProviderManifest {
	return &providerv1.ProviderManifest{
		Id:          ProviderID,
		Type:        "sandbox",
		Version:     "dev",
		DisplayName: "Sandbox",
		AgentSurface: &providerv1.AgentSurface{
			Protocol: "mcp",
		},
		SignalSurface: &providerv1.SignalSurface{
			Supported: true,
		},
		StateSurface: &providerv1.StateSurface{
			Supported: true,
		},
		ControlSurface: &providerv1.ControlSurface{
			Features: []string{
				"sandboxes",
				"workspaces",
			},
		},
		ObservationSurface: &providerv1.ObservationSurface{
			Events: []string{
				"sandbox.created",
				"sandbox.process.exited",
				"sandbox.artifact.discovered",
			},
		},
		ResourceSurface: &providerv1.ResourceSurface{
			ResourceTypes: []string{
				"sandbox_artifact",
				"sandbox_export",
				"workspace_snapshot",
			},
		},
		GovernanceSurface: &providerv1.GovernanceSurface{
			Supported: true,
		},
	}
}
