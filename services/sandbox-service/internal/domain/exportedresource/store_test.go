package exportedresource

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryStoreCreateGet(t *testing.T) {
	store := NewMemoryStore()
	record, err := store.Create(context.Background(), Record{
		ResourceID:               "res_1",
		BlobStoreKind:            "localblob",
		BlobID:                   "res_1",
		SourceServiceWorkspaceID: "ws_1",
		SourceWorkspacePath:      "outputs/report.txt",
		Name:                     "report.txt",
		MimeType:                 "text/plain",
		SizeBytes:                12,
		ContentHash:              "sha256:abc",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if record.CreatedAt.IsZero() {
		t.Fatal("Create() did not set CreatedAt")
	}

	got, err := store.Get(context.Background(), "res_1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ResourceID != "res_1" || got.BlobID != "res_1" || got.SourceWorkspacePath != "outputs/report.txt" || got.ContentHash != "sha256:abc" {
		t.Fatalf("Get() = %+v", got)
	}
}

func TestMemoryStoreDuplicateID(t *testing.T) {
	store := NewMemoryStore()
	record := Record{ResourceID: "res_1", BlobStoreKind: "localblob", BlobID: "res_1", SourceServiceWorkspaceID: "ws_1", SourceWorkspacePath: "a.txt", Name: "a.txt"}
	if _, err := store.Create(context.Background(), record); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := store.Create(context.Background(), record); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("Create() duplicate error = %v, want %v", err, ErrAlreadyExists)
	}
}

func TestMemoryStoreMissingID(t *testing.T) {
	store := NewMemoryStore()
	if _, err := store.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want %v", err, ErrNotFound)
	}
}
