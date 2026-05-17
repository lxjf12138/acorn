package localblob

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	resourceblob "github.com/lxjf12138/acorn/packages/core/resourceblob"
)

func TestStorePutOpenStatDelete(t *testing.T) {
	store := newTestStore(t)
	stored, err := store.Put(context.Background(), resourceblob.PutRequest{
		ResourceID: "res_1",
		Name:       "report.txt",
		MimeType:   "text/plain",
		Source:     strings.NewReader("hello"),
	})
	if err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	if stored.ResourceID != "res_1" || stored.StoreKind != Kind || stored.StoreBlobID != "res_1" {
		t.Fatalf("unexpected stored blob: %+v", stored)
	}
	if stored.SizeBytes != 5 {
		t.Fatalf("unexpected size: %d", stored.SizeBytes)
	}
	if stored.ContentHash != "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("unexpected hash: %q", stored.ContentHash)
	}
	if stored.CreatedAt.IsZero() {
		t.Fatal("CreatedAt is zero")
	}
	if _, err := os.Stat(filepath.Join(store.baseDir, "res_1.blob")); err != nil {
		t.Fatalf("blob file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.baseDir, "res_1.tmp")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("tmp file should not remain, err=%v", err)
	}

	info, err := store.Stat(context.Background(), "res_1")
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.ContentHash != stored.ContentHash || info.SizeBytes != stored.SizeBytes {
		t.Fatalf("unexpected stat info: %+v", info)
	}

	reader, info, err := store.Open(context.Background(), "res_1")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read blob: %v", err)
	}
	if string(body) != "hello" || info.Name != "report.txt" {
		t.Fatalf("unexpected open result body=%q info=%+v", string(body), info)
	}

	if err := store.Delete(context.Background(), "res_1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := store.Stat(context.Background(), "res_1"); !errors.Is(err, resourceblob.ErrNotFound) {
		t.Fatalf("expected NotFound after delete, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.baseDir, "res_1.blob")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("blob file should be deleted, err=%v", err)
	}
}

func TestStorePutDuplicateResourceID(t *testing.T) {
	store := newTestStore(t)
	req := resourceblob.PutRequest{ResourceID: "res_1", Source: strings.NewReader("one")}
	if _, err := store.Put(context.Background(), req); err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	if _, err := store.Put(context.Background(), resourceblob.PutRequest{ResourceID: "res_1", Source: strings.NewReader("two")}); !errors.Is(err, resourceblob.ErrAlreadyExists) {
		t.Fatalf("expected AlreadyExists, got %v", err)
	}
}

func TestStoreRejectsInvalidResourceID(t *testing.T) {
	store := newTestStore(t)
	for _, resourceID := range []string{"", "../res", "a/b", `a\b`, "res..1", "."} {
		t.Run(resourceID, func(t *testing.T) {
			_, err := store.Put(context.Background(), resourceblob.PutRequest{
				ResourceID: resourceID,
				Source:     strings.NewReader("hello"),
			})
			if !errors.Is(err, resourceblob.ErrInvalidResourceID) {
				t.Fatalf("expected InvalidResourceID, got %v", err)
			}
		})
	}
}

func TestStoreMissingBlob(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.Stat(context.Background(), "missing"); !errors.Is(err, resourceblob.ErrNotFound) {
		t.Fatalf("Stat expected NotFound, got %v", err)
	}
	if reader, _, err := store.Open(context.Background(), "missing"); !errors.Is(err, resourceblob.ErrNotFound) {
		if reader != nil {
			_ = reader.Close()
		}
		t.Fatalf("Open expected NotFound, got %v", err)
	}
	if err := store.Delete(context.Background(), "missing"); !errors.Is(err, resourceblob.ErrNotFound) {
		t.Fatalf("Delete expected NotFound, got %v", err)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(Config{BaseDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	return store
}
