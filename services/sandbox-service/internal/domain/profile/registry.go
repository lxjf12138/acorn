package profile

import (
	"sort"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/conf"
)

type Registry interface {
	Get(id string) (*Profile, error)
	Default() (*Profile, error)
	List() []*Profile
	ListEnabled() []*Profile
	HasCapability(profileID string, capability Capability) bool
	AnyEnabledHasCapability(capability Capability) bool
}

type MemoryRegistry struct {
	profiles []*Profile
	byID     map[string]*Profile
}

func NewMemoryRegistry(profiles []*Profile) *MemoryRegistry {
	out := &MemoryRegistry{
		byID: make(map[string]*Profile, len(profiles)),
	}
	for _, candidate := range profiles {
		if candidate == nil || candidate.ID == "" {
			continue
		}
		p := candidate.Clone()
		out.profiles = append(out.profiles, p)
		out.byID[p.ID] = p
	}
	sort.SliceStable(out.profiles, func(i, j int) bool {
		if out.profiles[i].Default != out.profiles[j].Default {
			return out.profiles[i].Default
		}
		return out.profiles[i].ID < out.profiles[j].ID
	})
	return out
}

func NewRegistryFromConfig(cfg conf.Sandbox) *MemoryRegistry {
	var profiles []*Profile
	if cfg.LocalProcess.Enabled {
		profiles = append(profiles, &Profile{
			ID:                 LocalProcessDevID,
			DisplayName:        "Local Process Dev Backend",
			Description:        "Runs commands as host processes for local development. Not a security boundary.",
			Enabled:            true,
			Default:            true,
			IsolationClass:     IsolationDevProcess,
			WorkspaceStoreKind: "localfs",
			AttachmentKind:     "local_path",
			BackendID:          LocalProcessDevID,
			Capabilities: []Capability{
				CapabilityWorkspaceView,
				CapabilityWorkspaceResource,
				CapabilityWorkspaceExec,
				CapabilityLocalProcessExec,
			},
			Metadata: map[string]string{
				"dev_only": "true",
			},
		})
	}
	return NewMemoryRegistry(profiles)
}

func (r *MemoryRegistry) Get(id string) (*Profile, error) {
	p, ok := r.byID[id]
	if !ok {
		return nil, ErrProfileNotFound
	}
	if !p.Enabled {
		return nil, ErrProfileDisabled
	}
	return p.Clone(), nil
}

func (r *MemoryRegistry) Default() (*Profile, error) {
	for _, p := range r.profiles {
		if p.Enabled && p.Default {
			return p.Clone(), nil
		}
	}
	return nil, ErrNoDefaultProfile
}

func (r *MemoryRegistry) List() []*Profile {
	return cloneProfiles(r.profiles)
}

func (r *MemoryRegistry) ListEnabled() []*Profile {
	var out []*Profile
	for _, p := range r.profiles {
		if p.Enabled {
			out = append(out, p.Clone())
		}
	}
	return out
}

func (r *MemoryRegistry) HasCapability(profileID string, capability Capability) bool {
	p, ok := r.byID[profileID]
	return ok && p.Enabled && p.HasCapability(capability)
}

func (r *MemoryRegistry) AnyEnabledHasCapability(capability Capability) bool {
	for _, p := range r.profiles {
		if p.Enabled && p.HasCapability(capability) {
			return true
		}
	}
	return false
}

func cloneProfiles(profiles []*Profile) []*Profile {
	out := make([]*Profile, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, p.Clone())
	}
	return out
}
