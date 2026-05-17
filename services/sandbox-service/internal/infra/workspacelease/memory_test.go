package workspacelease

import (
	"context"
	"errors"
	"testing"

	leasedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacelease"
)

func TestMemoryManagerReadLeasesCanCoexist(t *testing.T) {
	manager := NewMemoryManager()
	first := acquire(t, manager, "ws-1", leasedomain.ModeRead)
	second := acquire(t, manager, "ws-1", leasedomain.ModeRead)
	if first.ID == second.ID {
		t.Fatalf("expected different lease ids")
	}
}

func TestMemoryManagerWriteExclusion(t *testing.T) {
	tests := []struct {
		name   string
		first  leasedomain.Mode
		second leasedomain.Mode
	}{
		{name: "write excludes read", first: leasedomain.ModeWrite, second: leasedomain.ModeRead},
		{name: "read excludes write", first: leasedomain.ModeRead, second: leasedomain.ModeWrite},
		{name: "write excludes write", first: leasedomain.ModeWrite, second: leasedomain.ModeWrite},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewMemoryManager()
			_ = acquire(t, manager, "ws-1", tt.first)
			if _, err := manager.TryAcquire(context.Background(), leasedomain.AcquireRequest{WorkspaceID: "ws-1", Mode: tt.second}); !errors.Is(err, leasedomain.ErrWorkspaceBusy) {
				t.Fatalf("expected ErrWorkspaceBusy, got %v", err)
			}
		})
	}
}

func TestMemoryManagerRelease(t *testing.T) {
	manager := NewMemoryManager()
	read := acquire(t, manager, "ws-1", leasedomain.ModeRead)
	if err := manager.Release(context.Background(), read); err != nil {
		t.Fatalf("Release read returned error: %v", err)
	}
	write := acquire(t, manager, "ws-1", leasedomain.ModeWrite)
	if err := manager.Release(context.Background(), write); err != nil {
		t.Fatalf("Release write returned error: %v", err)
	}
	if err := manager.Release(context.Background(), write); !errors.Is(err, leasedomain.ErrLeaseNotFound) {
		t.Fatalf("expected ErrLeaseNotFound, got %v", err)
	}
}

func TestMemoryManagerDifferentWorkspacesDoNotBlock(t *testing.T) {
	manager := NewMemoryManager()
	_ = acquire(t, manager, "ws-1", leasedomain.ModeWrite)
	_ = acquire(t, manager, "ws-2", leasedomain.ModeWrite)
}

func TestMemoryManagerValidation(t *testing.T) {
	manager := NewMemoryManager()
	if _, err := manager.TryAcquire(context.Background(), leasedomain.AcquireRequest{Mode: leasedomain.ModeRead}); !errors.Is(err, leasedomain.ErrWorkspaceIDRequired) {
		t.Fatalf("expected ErrWorkspaceIDRequired, got %v", err)
	}
	if _, err := manager.TryAcquire(context.Background(), leasedomain.AcquireRequest{WorkspaceID: "ws-1", Mode: "bad"}); !errors.Is(err, leasedomain.ErrInvalidMode) {
		t.Fatalf("expected ErrInvalidMode, got %v", err)
	}
}

func acquire(t *testing.T, manager *MemoryManager, workspaceID string, mode leasedomain.Mode) *leasedomain.Lease {
	t.Helper()
	lease, err := manager.TryAcquire(context.Background(), leasedomain.AcquireRequest{
		WorkspaceID: workspaceID,
		Mode:        mode,
	})
	if err != nil {
		t.Fatalf("TryAcquire returned error: %v", err)
	}
	return lease
}
