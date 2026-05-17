package localfs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/attachment"
)

func TestWorkspaceStorePrepareLocalProcessAttachment(t *testing.T) {
	store := newTestWorkspaceStore(t)
	createBacking(t, store, "ws-test")

	att, err := store.Prepare(context.Background(), attachment.PrepareRequest{
		WorkspaceID: "ws-test",
		Target:      attachment.Target{Kind: attachment.TargetLocalProcess},
		ReadOnly:    true,
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	wantPath, err := filepath.EvalSymlinks(filepath.Join(store.baseDir, "ws-test"))
	if err != nil {
		t.Fatalf("EvalSymlinks workspace root: %v", err)
	}
	if att.ID == "" ||
		att.WorkspaceID != "ws-test" ||
		att.Kind != attachment.KindLocalPath ||
		att.LocalPath != wantPath ||
		!att.ReadOnly ||
		att.Metadata["store_kind"] != Kind {
		t.Fatalf("unexpected attachment: %+v", att)
	}
}

func TestWorkspaceStorePrepareAttachmentErrors(t *testing.T) {
	store := newTestWorkspaceStore(t)
	createBacking(t, store, "ws-test")

	tests := []struct {
		name string
		req  attachment.PrepareRequest
		err  error
	}{
		{name: "missing workspace id", req: attachment.PrepareRequest{}, err: attachment.ErrWorkspaceNotFound},
		{name: "missing workspace", req: attachment.PrepareRequest{WorkspaceID: "missing", Target: attachment.Target{Kind: attachment.TargetLocalProcess}}, err: attachment.ErrWorkspaceNotFound},
		{name: "missing target", req: attachment.PrepareRequest{WorkspaceID: "ws-test"}, err: attachment.ErrUnsupportedTarget},
		{name: "unsupported target", req: attachment.PrepareRequest{WorkspaceID: "ws-test", Target: attachment.Target{Kind: attachment.TargetDocker}}, err: attachment.ErrUnsupportedTarget},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := store.Prepare(context.Background(), tt.req)
			if !errors.Is(err, tt.err) {
				t.Fatalf("expected %v, got %v", tt.err, err)
			}
		})
	}
}

func TestWorkspaceStorePrepareAttachmentRootSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior is platform dependent on Windows")
	}
	store := newTestWorkspaceStore(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(store.baseDir, "ws-escape")); err != nil {
		t.Fatalf("symlink escape root: %v", err)
	}

	_, err := store.Prepare(context.Background(), attachment.PrepareRequest{
		WorkspaceID: "ws-escape",
		Target:      attachment.Target{Kind: attachment.TargetLocalProcess},
	})
	if !errors.Is(err, attachment.ErrAttachmentNotReady) {
		t.Fatalf("expected attachment not ready, got %v", err)
	}
}

func TestWorkspaceStorePrepareAttachmentRootNotDirectory(t *testing.T) {
	store := newTestWorkspaceStore(t)
	writeFile(t, store.baseDir, "ws-file", "not a directory")

	_, err := store.Prepare(context.Background(), attachment.PrepareRequest{
		WorkspaceID: "ws-file",
		Target:      attachment.Target{Kind: attachment.TargetLocalProcess},
	})
	if !errors.Is(err, attachment.ErrAttachmentNotReady) {
		t.Fatalf("expected attachment not ready, got %v", err)
	}
}

func TestWorkspaceStoreReleaseAttachment(t *testing.T) {
	store := newTestWorkspaceStore(t)
	if err := store.Release(context.Background(), &attachment.WorkspaceAttachment{ID: "att_ws"}); err != nil {
		t.Fatalf("Release returned error: %v", err)
	}
}
