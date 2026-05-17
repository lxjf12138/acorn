package sandboxpolicy

import (
	"context"
	"errors"
	"testing"

	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/conf"
)

func TestConfigResolverResolveWorkspaceProfile(t *testing.T) {
	available := set("local-process-dev", "cloud-vm", "cloud-vm-gpu")
	tests := []struct {
		name        string
		policies    conf.SandboxPolicies
		legacy      string
		req         ResolveWorkspaceProfileRequest
		wantProfile string
		wantSource  string
		wantErr     error
	}{
		{
			name:        "global default selected",
			policies:    conf.SandboxPolicies{Global: conf.SandboxPolicyConfig{DefaultProfileID: "local-process-dev"}},
			req:         ResolveWorkspaceProfileRequest{AvailableProfileIDs: available},
			wantProfile: "local-process-dev",
			wantSource:  SourceGlobalDefault,
		},
		{
			name: "tenant default overrides global",
			policies: conf.SandboxPolicies{
				Global:  conf.SandboxPolicyConfig{DefaultProfileID: "local-process-dev", AllowedProfileIDs: []string{"cloud-vm"}},
				Tenants: map[string]conf.SandboxPolicyConfig{"t1": {DefaultProfileID: "cloud-vm"}},
			},
			req:         ResolveWorkspaceProfileRequest{TenantID: "t1", AvailableProfileIDs: available},
			wantProfile: "cloud-vm",
			wantSource:  SourceTenantDefault,
		},
		{
			name: "user default overrides tenant",
			policies: conf.SandboxPolicies{
				Global:  conf.SandboxPolicyConfig{DefaultProfileID: "local-process-dev"},
				Tenants: map[string]conf.SandboxPolicyConfig{"t1": {DefaultProfileID: "cloud-vm"}},
				Users:   map[string]conf.SandboxPolicyConfig{"u1": {DefaultProfileID: "cloud-vm-gpu"}},
			},
			req:         ResolveWorkspaceProfileRequest{TenantID: "t1", UserID: "u1", AvailableProfileIDs: available},
			wantProfile: "cloud-vm-gpu",
			wantSource:  SourceUserDefault,
		},
		{
			name:        "requested allowed succeeds",
			policies:    conf.SandboxPolicies{Global: conf.SandboxPolicyConfig{DefaultProfileID: "local-process-dev", AllowedProfileIDs: []string{"local-process-dev", "cloud-vm"}}},
			req:         ResolveWorkspaceProfileRequest{RequestedProfileID: "cloud-vm", AvailableProfileIDs: available},
			wantProfile: "cloud-vm",
			wantSource:  SourceRequested,
		},
		{
			name:     "requested not allowed",
			policies: conf.SandboxPolicies{Global: conf.SandboxPolicyConfig{DefaultProfileID: "local-process-dev"}},
			req:      ResolveWorkspaceProfileRequest{RequestedProfileID: "cloud-vm", AvailableProfileIDs: available},
			wantErr:  ErrProfileNotAllowed,
		},
		{
			name:     "requested unavailable",
			policies: conf.SandboxPolicies{Global: conf.SandboxPolicyConfig{DefaultProfileID: "local-process-dev", AllowedProfileIDs: []string{"cloud-vm"}}},
			req:      ResolveWorkspaceProfileRequest{RequestedProfileID: "cloud-vm", AvailableProfileIDs: set("local-process-dev")},
			wantErr:  ErrProfileUnavailable,
		},
		{
			name:     "default unavailable",
			policies: conf.SandboxPolicies{Global: conf.SandboxPolicyConfig{DefaultProfileID: "cloud-vm"}},
			req:      ResolveWorkspaceProfileRequest{AvailableProfileIDs: set("local-process-dev")},
			wantErr:  ErrProfileUnavailable,
		},
		{
			name:     "allowed empty means only default allowed",
			policies: conf.SandboxPolicies{Global: conf.SandboxPolicyConfig{DefaultProfileID: "local-process-dev"}},
			req:      ResolveWorkspaceProfileRequest{RequestedProfileID: "cloud-vm", AvailableProfileIDs: available},
			wantErr:  ErrProfileNotAllowed,
		},
		{
			name:        "legacy fallback",
			legacy:      "local-process-dev",
			req:         ResolveWorkspaceProfileRequest{AvailableProfileIDs: available},
			wantProfile: "local-process-dev",
			wantSource:  SourceLegacyDefault,
		},
		{
			name:    "no default",
			req:     ResolveWorkspaceProfileRequest{AvailableProfileIDs: available},
			wantErr: ErrNoProfileSelected,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewConfigResolver(tt.policies, tt.legacy)
			got, err := resolver.ResolveWorkspaceProfile(context.Background(), tt.req)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveWorkspaceProfile returned error: %v", err)
			}
			if got.ProfileID != tt.wantProfile || got.Source != tt.wantSource {
				t.Fatalf("unexpected result: %+v", got)
			}
		})
	}
}

func set(values ...string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}
