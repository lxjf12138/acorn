package workspace

import (
	"context"
	"errors"
	"testing"
	"time"

	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
)

func TestMemoryStoreCreateAndGet(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now()
	created, err := store.Create(context.Background(), Workspace{
		SandboxProfileID: "local-process",
		DisplayName:      "test workspace",
		Status:           workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
		CreatedAt:        now,
		UpdatedAt:        now,
		MetadataJSON:     []byte(`{"a":1}`),
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID == "" {
		t.Fatal("Create did not assign ID")
	}

	got, err := store.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ID != created.ID || got.SandboxProfileID != "local-process" {
		t.Fatalf("unexpected workspace: %+v", got)
	}
	got.MetadataJSON[0] = '{'
	again, err := store.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if string(again.MetadataJSON) != `{"a":1}` {
		t.Fatalf("store returned mutable metadata: %s", again.MetadataJSON)
	}
}

func TestMemoryStoreGetMissing(t *testing.T) {
	store := NewMemoryStore()
	_, err := store.Get(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
