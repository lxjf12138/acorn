package descriptor

import (
	"context"
	"encoding/json"

	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/conf"
	profiledomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/profile"
)

const (
	Kind     = "sandbox"
	Contract = "acorn.sandbox"
)

type Options struct {
	ServiceID       string
	DisplayName     string
	Version         string
	HTTPAddr        string
	GRPCAddr        string
	MCPEndpoint     string
	ProfileRegistry profiledomain.Registry
}

type Source struct {
	opts Options
}

func NewSource(opts Options) *Source {
	return &Source{opts: opts.withDefaults()}
}

func NewSourceFromConfig(cfg *conf.Config, version string, profiles profiledomain.Registry) *Source {
	if profiles == nil {
		profiles = profiledomain.NewRegistryFromConfig(cfg.Sandbox)
	}
	return NewSource(Options{
		ServiceID:       cfg.Service.ID,
		DisplayName:     cfg.Service.Name,
		Version:         version,
		HTTPAddr:        cfg.Server.HTTP.AdvertiseAddr,
		GRPCAddr:        cfg.Server.GRPC.AdvertiseAddr,
		ProfileRegistry: profiles,
	})
}

func (s *Source) DescribeCapabilities(context.Context) (*capabilityv1.CapabilityDescriptor, error) {
	controlFeatures := []string{
		"describe_capabilities",
		"health",
		"version",
		"create_hosted_workspace",
		"get_hosted_workspace",
	}
	profiles := s.sandboxProfiles()
	endpoints := []*capabilityv1.EndpointDescriptor{
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
		{
			Name:      "workspace-transfer-grpc",
			Surface:   "resource",
			Protocol:  "grpc",
			Transport: "grpc",
			Address:   s.opts.GRPCAddr,
			Path:      "/acorn.sandbox.v1.WorkspaceTransferService",
			Status:    capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL,
		},
		{
			Name:      "resource-content-grpc",
			Surface:   "resource",
			Protocol:  "grpc",
			Transport: "grpc",
			Address:   s.opts.GRPCAddr,
			Path:      "/acorn.resource.v1.ResourceContentService",
			Status:    capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL,
		},
	}
	if s.hasEnabledProfileCapability(profiledomain.CapabilityWorkspaceExec) {
		controlFeatures = append(controlFeatures, "exec_workspace_command")
		endpoints = append(endpoints, &capabilityv1.EndpointDescriptor{
			Name:      "workspace-exec-grpc",
			Surface:   "control",
			Protocol:  "grpc",
			Transport: "grpc",
			Address:   s.opts.GRPCAddr,
			Path:      "/acorn.sandbox.v1.WorkspaceExecService",
			Status:    capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL,
		})
	}
	return &capabilityv1.CapabilityDescriptor{
		ServiceId:   s.opts.ServiceID,
		Kind:        Kind,
		Contract:    Contract,
		Version:     s.opts.Version,
		DisplayName: s.opts.DisplayName,
		Description: "Sandbox capability service descriptor for Phase 1 capability discovery.",
		Surfaces: []*capabilityv1.SurfaceDescriptor{
			{
				Name:     "control",
				Status:   capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL,
				Features: controlFeatures,
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
				Status: capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL,
				Features: []string{
					"export_workspace_path",
					"open_resource",
					"import_resource",
				},
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
		SandboxProfiles: profiles,
		Endpoints:       endpoints,
	}, nil
}

func (s *Source) SandboxProfile(id string) (*capabilityv1.SandboxProfile, bool) {
	if s.opts.ProfileRegistry == nil {
		return nil, false
	}
	p, err := s.opts.ProfileRegistry.Get(id)
	if err != nil {
		return nil, false
	}
	return toSandboxProfileDescriptor(p), true
}

func (s *Source) DefaultSandboxProfile() (*capabilityv1.SandboxProfile, bool) {
	if s.opts.ProfileRegistry == nil {
		return nil, false
	}
	p, err := s.opts.ProfileRegistry.Default()
	if err != nil {
		return nil, false
	}
	return toSandboxProfileDescriptor(p), true
}

func (s *Source) sandboxProfiles() []*capabilityv1.SandboxProfile {
	if s.opts.ProfileRegistry == nil {
		return nil
	}
	profiles := s.opts.ProfileRegistry.ListEnabled()
	out := make([]*capabilityv1.SandboxProfile, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, toSandboxProfileDescriptor(p))
	}
	return out
}

func (s *Source) hasEnabledProfileCapability(capability profiledomain.Capability) bool {
	return s.opts.ProfileRegistry != nil && s.opts.ProfileRegistry.AnyEnabledHasCapability(capability)
}

func toSandboxProfileDescriptor(p *profiledomain.Profile) *capabilityv1.SandboxProfile {
	return &capabilityv1.SandboxProfile{
		Id:             p.ID,
		DisplayName:    p.DisplayName,
		Implementation: p.BackendID,
		Isolation:      string(p.IsolationClass),
		Os:             "host",
		Default:        p.Default,
		Status:         capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL,
		Capabilities:   profileCapabilities(p),
		MetadataJson:   profileMetadataJSON(p),
	}
}

func profileCapabilities(p *profiledomain.Profile) []string {
	out := make([]string, 0, len(p.Capabilities))
	for _, capability := range p.Capabilities {
		out = append(out, string(capability))
	}
	if p.Metadata["dev_only"] == "true" {
		out = append(out, "dev_only")
	}
	return out
}

func profileMetadataJSON(p *profiledomain.Profile) []byte {
	if p.Metadata == nil {
		return nil
	}
	data, err := json.Marshal(p.Metadata)
	if err != nil {
		return nil
	}
	return data
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
