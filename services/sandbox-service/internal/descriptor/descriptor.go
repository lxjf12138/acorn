package descriptor

import (
	"context"

	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/conf"
)

const (
	Kind     = "sandbox"
	Contract = "acorn.sandbox"
)

type Options struct {
	ServiceID   string
	DisplayName string
	Version     string
	HTTPAddr    string
	GRPCAddr    string
	MCPEndpoint string
}

type Source struct {
	opts Options
}

func NewSource(opts Options) *Source {
	return &Source{opts: opts.withDefaults()}
}

func NewSourceFromConfig(cfg *conf.Config, version string) *Source {
	return NewSource(Options{
		ServiceID:   cfg.Service.ID,
		DisplayName: cfg.Service.Name,
		Version:     version,
		HTTPAddr:    cfg.Server.HTTP.AdvertiseAddr,
		GRPCAddr:    cfg.Server.GRPC.AdvertiseAddr,
	})
}

func (s *Source) DescribeCapabilities(context.Context) (*capabilityv1.CapabilityDescriptor, error) {
	return &capabilityv1.CapabilityDescriptor{
		ServiceId:   s.opts.ServiceID,
		Kind:        Kind,
		Contract:    Contract,
		Version:     s.opts.Version,
		DisplayName: s.opts.DisplayName,
		Description: "Sandbox capability service descriptor for Phase 1 capability discovery.",
		Surfaces: []*capabilityv1.SurfaceDescriptor{
			{
				Name:   "control",
				Status: capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL,
				Features: []string{
					"describe_capabilities",
					"health",
					"version",
					"create_hosted_workspace",
					"get_hosted_workspace",
				},
			},
			{
				Name:   "agent",
				Status: capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED,
			},
			{
				Name:   "state",
				Status: capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL,
				Features: []string{
					"workspace_state",
				},
			},
			{
				Name:   "view",
				Status: capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL,
				Features: []string{
					"list_workspace_dir",
					"preview_workspace_file",
				},
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
					Status:    capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED,
					Transport: "http",
					Endpoint:  s.opts.MCPEndpoint,
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
		Endpoints: []*capabilityv1.EndpointDescriptor{
			{
				Name:      "control-http",
				Surface:   "control",
				Protocol:  "http",
				Transport: "http",
				Address:   s.opts.HTTPAddr,
				Path:      "/capability/descriptor",
				Status:    capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_IMPLEMENTED,
			},
			{
				Name:      "control-grpc",
				Surface:   "control",
				Protocol:  "grpc",
				Transport: "grpc",
				Address:   s.opts.GRPCAddr,
				Path:      "/acorn.capability.v1.CapabilityDescriptorService/GetCapabilityDescriptor",
				Status:    capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_IMPLEMENTED,
			},
			{
				Name:      "workspace-host-grpc",
				Surface:   "control",
				Protocol:  "grpc",
				Transport: "grpc",
				Address:   s.opts.GRPCAddr,
				Path:      "/acorn.sandbox.v1.WorkspaceHostService",
				Status:    capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL,
			},
			{
				Name:      "agent-mcp",
				Surface:   "agent",
				Protocol:  "mcp",
				Transport: "http",
				Address:   s.opts.HTTPAddr,
				Path:      s.opts.MCPEndpoint,
				Status:    capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED,
			},
			{
				Name:      "workspace-view-grpc",
				Surface:   "view",
				Protocol:  "grpc",
				Transport: "grpc",
				Address:   s.opts.GRPCAddr,
				Path:      "/acorn.sandbox.v1.WorkspaceViewService",
				Status:    capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL,
			},
		},
	}, nil
}

func (s *Source) SandboxProfile(id string) (*capabilityv1.SandboxProfile, bool) {
	descriptor, err := s.DescribeCapabilities(context.Background())
	if err != nil {
		return nil, false
	}
	for _, profile := range descriptor.GetSandboxProfiles() {
		if profile.GetId() == id {
			return profile, true
		}
	}
	return nil, false
}

func (s *Source) DefaultSandboxProfile() (*capabilityv1.SandboxProfile, bool) {
	descriptor, err := s.DescribeCapabilities(context.Background())
	if err != nil {
		return nil, false
	}
	for _, profile := range descriptor.GetSandboxProfiles() {
		if profile.GetDefault() {
			return profile, true
		}
	}
	return nil, false
}

func (o Options) withDefaults() Options {
	if o.ServiceID == "" {
		o.ServiceID = "sandbox-service"
	}
	if o.DisplayName == "" {
		o.DisplayName = "Local Sandbox Service"
	}
	if o.Version == "" {
		o.Version = "dev"
	}
	if o.MCPEndpoint == "" {
		o.MCPEndpoint = "/mcp"
	}
	return o
}
