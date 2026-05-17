package sandboxpolicy

import (
	"context"

	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/conf"
)

const (
	SourceRequested     = "requested"
	SourceUserDefault   = "user_default"
	SourceTenantDefault = "tenant_default"
	SourceGlobalDefault = "global_default"
	SourceLegacyDefault = "legacy_default"
)

type Resolver interface {
	ResolveWorkspaceProfile(ctx context.Context, req ResolveWorkspaceProfileRequest) (*ResolveWorkspaceProfileResult, error)
}

type ConfigResolver struct {
	global  SandboxPolicyConfig
	tenants map[string]SandboxPolicyConfig
	users   map[string]SandboxPolicyConfig

	legacyDefaultProfileID string
}

type SandboxPolicyConfig struct {
	DefaultProfileID  string
	AllowedProfileIDs []string
}

func NewConfigResolver(policies conf.SandboxPolicies, legacyDefaultProfileID string) *ConfigResolver {
	return &ConfigResolver{
		global:                 fromConfig(policies.Global),
		tenants:                fromConfigMap(policies.Tenants),
		users:                  fromConfigMap(policies.Users),
		legacyDefaultProfileID: legacyDefaultProfileID,
	}
}

func (r *ConfigResolver) ResolveWorkspaceProfile(_ context.Context, req ResolveWorkspaceProfileRequest) (*ResolveWorkspaceProfileResult, error) {
	effective := r.effectivePolicy(req.TenantID, req.UserID)
	if req.RequestedProfileID != "" {
		return r.validateSelection(req.RequestedProfileID, SourceRequested, effective, req.AvailableProfileIDs)
	}
	for _, candidate := range []struct {
		profileID string
		source    string
	}{
		{profileID: r.users[req.UserID].DefaultProfileID, source: SourceUserDefault},
		{profileID: r.tenants[req.TenantID].DefaultProfileID, source: SourceTenantDefault},
		{profileID: r.global.DefaultProfileID, source: SourceGlobalDefault},
		{profileID: r.legacyDefaultProfileID, source: SourceLegacyDefault},
	} {
		if candidate.profileID != "" {
			if candidate.source == SourceLegacyDefault && len(effective.AllowedProfileIDs) == 0 && effective.DefaultProfileID == "" {
				effective.DefaultProfileID = candidate.profileID
			}
			return r.validateSelection(candidate.profileID, candidate.source, effective, req.AvailableProfileIDs)
		}
	}
	return nil, ErrNoProfileSelected
}

func (r *ConfigResolver) effectivePolicy(tenantID string, userID string) SandboxPolicyConfig {
	effective := r.global
	if tenant, ok := r.tenants[tenantID]; ok {
		effective = mergePolicy(effective, tenant)
	}
	if user, ok := r.users[userID]; ok {
		effective = mergePolicy(effective, user)
	}
	return effective
}

func (r *ConfigResolver) validateSelection(profileID string, source string, policy SandboxPolicyConfig, available map[string]struct{}) (*ResolveWorkspaceProfileResult, error) {
	if profileID == "" {
		return nil, ErrNoProfileSelected
	}
	if !isAllowed(profileID, policy) {
		return nil, ErrProfileNotAllowed
	}
	if _, ok := available[profileID]; !ok {
		return nil, ErrProfileUnavailable
	}
	return &ResolveWorkspaceProfileResult{ProfileID: profileID, Source: source}, nil
}

func mergePolicy(base SandboxPolicyConfig, override SandboxPolicyConfig) SandboxPolicyConfig {
	out := base
	if override.DefaultProfileID != "" {
		out.DefaultProfileID = override.DefaultProfileID
	}
	out.AllowedProfileIDs = append([]string(nil), override.AllowedProfileIDs...)
	return out
}

func isAllowed(profileID string, policy SandboxPolicyConfig) bool {
	allowed := allowedSet(policy)
	_, ok := allowed[profileID]
	return ok
}

func allowedSet(policy SandboxPolicyConfig) map[string]struct{} {
	out := map[string]struct{}{}
	if len(policy.AllowedProfileIDs) > 0 {
		for _, profileID := range policy.AllowedProfileIDs {
			if profileID != "" {
				out[profileID] = struct{}{}
			}
		}
		return out
	}
	if policy.DefaultProfileID != "" {
		out[policy.DefaultProfileID] = struct{}{}
	}
	return out
}

func fromConfig(in conf.SandboxPolicyConfig) SandboxPolicyConfig {
	return SandboxPolicyConfig{
		DefaultProfileID:  in.DefaultProfileID,
		AllowedProfileIDs: append([]string(nil), in.AllowedProfileIDs...),
	}
}

func fromConfigMap(in map[string]conf.SandboxPolicyConfig) map[string]SandboxPolicyConfig {
	out := make(map[string]SandboxPolicyConfig, len(in))
	for key, value := range in {
		out[key] = fromConfig(value)
	}
	return out
}
