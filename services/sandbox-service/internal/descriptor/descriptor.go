package descriptor

import (
	"context"

	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
)

const ServiceID = "sandbox.local.dev"

type Source struct{}

func NewSource() *Source {
	return &Source{}
}

func (s *Source) DescribeCapabilities(context.Context) (*capabilityv1.CapabilityDescriptor, error) {
	return &capabilityv1.CapabilityDescriptor{
		ServiceId:   ServiceID,
		Kind:        "sandbox",
		Contract:    "acorn.sandbox",
		Version:     "dev",
		DisplayName: "Local Sandbox Service",
		Description: "Sandbox capability service descriptor for Phase 1 capability discovery.",
		Surfaces: []*capabilityv1.SurfaceDescriptor{
			{
				Name:   "control",
				Status: capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_IMPLEMENTED,
				Features: []string{
					"describe_capabilities",
					"health",
					"version",
				},
			},
			{
				Name:   "agent",
				Status: capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED,
			},
			{
				Name:   "state",
				Status: capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED,
			},
			{
				Name:   "resource",
				Status: capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED,
			},
			{
				Name:   "signal",
				Status: capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED,
			},
			{
				Name:   "observation",
				Status: capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED,
			},
			{
				Name:   "governance",
				Status: capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED,
			},
		},
		AgentSurface: &capabilityv1.AgentSurfaceDescriptor{
			Status: capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED,
			Mcp: []*capabilityv1.MCPAgentSurface{
				{
					Transport: "http",
					Endpoint:  "",
					Tools:     nil,
				},
			},
		},
		SandboxProfiles: []*capabilityv1.SandboxProfile{
			{
				Id:             "local-process",
				DisplayName:    "Local Process Sandbox",
				Implementation: "local-process",
				Isolation:      "process",
				Os:             "host",
				Default:        true,
				Status:         capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED,
				Capabilities: []string{
					"filesystem",
					"exec",
				},
			},
			{
				Id:             "local-docker",
				DisplayName:    "Local Docker Sandbox",
				Implementation: "local-docker",
				Isolation:      "container",
				Os:             "linux",
				Default:        false,
				Status:         capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED,
				Capabilities: []string{
					"filesystem",
					"exec",
					"network_optional",
				},
			},
		},
	}, nil
}
