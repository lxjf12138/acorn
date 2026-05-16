package workspace

import (
	"context"
	"errors"
	"testing"
	"time"

	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
)

func TestMemoryStoreCreateAndGetBySession(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now()
	created, err := store.Create(context.Background(), Record{
		SessionID:   "sess-1",
		OwnerUserID: "user-1",
		Status:      workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
		CurrentHost: &workspacev1.WorkspaceHostRef{
			ServiceId:          "sandbox-service",
			ServiceWorkspaceId: "ws-1",
			SandboxProfileId:   "local-process",
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID == "" {
		t.Fatal("Create did not assign ID")
	}
	got, ok, err := store.GetBySession(context.Background(), "sess-1")
	if err != nil || !ok {
		t.Fatalf("GetBySession returned (%+v, %v, %v)", got, ok, err)
	}
	if got.ID != created.ID || got.CurrentHost.GetServiceWorkspaceId() != "ws-1" {
		t.Fatalf("unexpected record: %+v", got)
	}
}

func TestMemoryStoreCreateIsIdempotentBySession(t *testing.T) {
	store := NewMemoryStore()
	first, err := store.Create(context.Background(), Record{
		SessionID:   "sess-1",
		OwnerUserID: "user-1",
		Status:      workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
		CurrentHost: &workspacev1.WorkspaceHostRef{
			ServiceId:          "sandbox-service",
			ServiceWorkspaceId: "ws-1",
			SandboxProfileId:   "local-process",
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	second, err := store.Create(context.Background(), Record{
		SessionID:   "sess-1",
		OwnerUserID: "user-1",
		Status:      workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
		CurrentHost: &workspacev1.WorkspaceHostRef{
			ServiceId:          "sandbox-service",
			ServiceWorkspaceId: "ws-2",
			SandboxProfileId:   "local-docker",
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if first.ID != second.ID || second.CurrentHost.GetServiceWorkspaceId() != "ws-1" {
		t.Fatalf("Create was not idempotent: first=%+v second=%+v", first, second)
	}
}

func TestMemoryStoreDifferentSessionsCreateDifferentRecords(t *testing.T) {
	store := NewMemoryStore()
	first, err := store.Create(context.Background(), Record{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	second, err := store.Create(context.Background(), Record{SessionID: "sess-2"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("different sessions shared record id: %q", first.ID)
	}
}

func TestMemoryStoreGetMissing(t *testing.T) {
	store := NewMemoryStore()
	_, err := store.Get(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
