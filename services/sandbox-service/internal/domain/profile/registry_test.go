package profile

import (
	"errors"
	"testing"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/conf"
)

func TestNewRegistryFromConfigLocalProcessEnabled(t *testing.T) {
	registry := NewRegistryFromConfig(conf.Sandbox{
		LocalProcess: conf.LocalProcess{Enabled: true},
	})
	p, err := registry.Get(LocalProcessDevID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if p.ID != LocalProcessDevID || !p.Enabled || !p.Default || p.BackendID != LocalProcessDevID {
		t.Fatalf("unexpected profile: %+v", p)
	}
	if !registry.HasCapability(LocalProcessDevID, CapabilityWorkspaceExec) ||
		!registry.HasCapability(LocalProcessDevID, CapabilityLocalProcessExec) {
		t.Fatalf("expected local process exec capabilities: %+v", p.Capabilities)
	}
}

func TestNewRegistryFromConfigLocalProcessDisabled(t *testing.T) {
	registry := NewRegistryFromConfig(conf.Sandbox{})
	if _, err := registry.Get(LocalProcessDevID); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound, got %v", err)
	}
	if _, err := registry.Default(); !errors.Is(err, ErrNoDefaultProfile) {
		t.Fatalf("expected ErrNoDefaultProfile, got %v", err)
	}
	if len(registry.ListEnabled()) != 0 {
		t.Fatalf("expected no enabled profiles: %+v", registry.ListEnabled())
	}
}

func TestMemoryRegistry(t *testing.T) {
	registry := NewMemoryRegistry([]*Profile{
		{
			ID:      "disabled",
			Enabled: false,
		},
		{
			ID:           "enabled",
			Enabled:      true,
			Default:      true,
			Capabilities: []Capability{CapabilityWorkspaceView},
		},
	})
	if _, err := registry.Get("missing"); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound, got %v", err)
	}
	if _, err := registry.Get("disabled"); !errors.Is(err, ErrProfileDisabled) {
		t.Fatalf("expected ErrProfileDisabled, got %v", err)
	}
	if p, err := registry.Default(); err != nil || p.ID != "enabled" {
		t.Fatalf("unexpected default: profile=%+v err=%v", p, err)
	}
	if !registry.HasCapability("enabled", CapabilityWorkspaceView) {
		t.Fatal("expected workspace view capability")
	}
	if registry.HasCapability("disabled", CapabilityWorkspaceView) {
		t.Fatal("disabled profile should not expose capability")
	}
	if !registry.AnyEnabledHasCapability(CapabilityWorkspaceView) {
		t.Fatal("expected enabled workspace view capability")
	}
}
