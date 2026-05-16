package service

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	exporteddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/exportedresource"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWorkspaceTransferServiceExportWorkspacePath(t *testing.T) {
	transfer, exportStore, workspace := newTestTransferService(t)
	writeFile(t, workspace.RootPath, "outputs/report.txt", "report")

	resp, err := transfer.ExportWorkspacePath(context.Background(), &sandboxv1.ExportWorkspacePathRequest{
		ServiceWorkspaceId: workspace.ID,
		Path:               "outputs/report.txt",
		ResourceName:       "custom.txt",
	})
	if err != nil {
		t.Fatalf("ExportWorkspacePath returned error: %v", err)
	}
	assertWorkspacePathRef(t, resp.GetSource(), workspace, "outputs/report.txt")
	if resp.GetResource().GetId() == "" {
		t.Fatal("resource id is empty")
	}
	if resp.GetResource().GetAuthorityServiceId() != "sandbox-service" {
		t.Fatalf("unexpected authority service id: %q", resp.GetResource().GetAuthorityServiceId())
	}
	if resp.GetResource().GetName() != "custom.txt" {
		t.Fatalf("unexpected resource name: %q", resp.GetResource().GetName())
	}
	if resp.GetResource().GetMimeType() != "text/plain; charset=utf-8" {
		t.Fatalf("unexpected mime type: %q", resp.GetResource().GetMimeType())
	}
	if resp.GetResource().GetSizeBytes() != int64(len("report")) {
		t.Fatalf("unexpected resource size: %d", resp.GetResource().GetSizeBytes())
	}
	record, err := exportStore.Get(context.Background(), resp.GetResource().GetId())
	if err != nil {
		t.Fatalf("export store Get returned error: %v", err)
	}
	if record.ServiceWorkspaceID != workspace.ID || record.WorkspacePath != "outputs/report.txt" {
		t.Fatalf("unexpected export record: %+v", record)
	}
}

func TestWorkspaceTransferServiceExportWorkspacePathDefaultNameAndMime(t *testing.T) {
	transfer, _, workspace := newTestTransferService(t)
	writeFile(t, workspace.RootPath, "outputs/blob.unknown", "blob")

	resp, err := transfer.ExportWorkspacePath(context.Background(), &sandboxv1.ExportWorkspacePathRequest{
		ServiceWorkspaceId: workspace.ID,
		Path:               "outputs/blob.unknown",
	})
	if err != nil {
		t.Fatalf("ExportWorkspacePath returned error: %v", err)
	}
	if resp.GetResource().GetName() != "blob.unknown" {
		t.Fatalf("unexpected default name: %q", resp.GetResource().GetName())
	}
	if resp.GetResource().GetMimeType() != "application/octet-stream" {
		t.Fatalf("unexpected default mime type: %q", resp.GetResource().GetMimeType())
	}
}

func TestWorkspaceTransferServiceExportWorkspacePathErrors(t *testing.T) {
	transfer, _, workspace := newTestTransferService(t)
	writeFile(t, workspace.RootPath, "file.txt", "content")
	if err := os.Mkdir(filepath.Join(workspace.RootPath, "dir"), 0o700); err != nil {
		t.Fatalf("mkdir dir: %v", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(workspace.RootPath, "file.txt"), filepath.Join(workspace.RootPath, "link.txt")); err != nil {
			t.Fatalf("symlink file: %v", err)
		}
		outside := t.TempDir()
		if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600); err != nil {
			t.Fatalf("write outside secret: %v", err)
		}
		if err := os.Symlink(outside, filepath.Join(workspace.RootPath, "linkdir")); err != nil {
			t.Fatalf("symlink outside dir: %v", err)
		}
	}

	tests := []struct {
		name string
		req  *sandboxv1.ExportWorkspacePathRequest
		code codes.Code
	}{
		{name: "missing workspace id", req: &sandboxv1.ExportWorkspacePathRequest{Path: "file.txt"}, code: codes.InvalidArgument},
		{name: "missing workspace", req: &sandboxv1.ExportWorkspacePathRequest{ServiceWorkspaceId: "missing", Path: "file.txt"}, code: codes.NotFound},
		{name: "traversal", req: &sandboxv1.ExportWorkspacePathRequest{ServiceWorkspaceId: workspace.ID, Path: "../secret"}, code: codes.InvalidArgument},
		{name: "directory", req: &sandboxv1.ExportWorkspacePathRequest{ServiceWorkspaceId: workspace.ID, Path: "dir"}, code: codes.FailedPrecondition},
		{name: "missing file", req: &sandboxv1.ExportWorkspacePathRequest{ServiceWorkspaceId: workspace.ID, Path: "missing.txt"}, code: codes.NotFound},
	}
	if runtime.GOOS != "windows" {
		tests = append(tests, struct {
			name string
			req  *sandboxv1.ExportWorkspacePathRequest
			code codes.Code
		}{name: "symlink", req: &sandboxv1.ExportWorkspacePathRequest{ServiceWorkspaceId: workspace.ID, Path: "link.txt"}, code: codes.PermissionDenied})
		tests = append(tests, struct {
			name string
			req  *sandboxv1.ExportWorkspacePathRequest
			code codes.Code
		}{name: "parent symlink escape", req: &sandboxv1.ExportWorkspacePathRequest{ServiceWorkspaceId: workspace.ID, Path: "linkdir/secret.txt"}, code: codes.PermissionDenied})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := transfer.ExportWorkspacePath(context.Background(), tt.req)
			if status.Code(err) != tt.code {
				t.Fatalf("expected %s, got %v", tt.code, err)
			}
		})
	}
}

func TestWorkspaceTransferServiceRejectsEmptyRootPath(t *testing.T) {
	store := workspacedomain.NewMemoryStore()
	workspace := workspacedomain.Workspace{
		ID:               "ws-empty-root",
		SandboxProfileID: "local-process",
		Status:           workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
	}
	if _, err := store.Create(context.Background(), workspace); err != nil {
		t.Fatalf("Create workspace returned error: %v", err)
	}
	transfer := NewWorkspaceTransferService("sandbox-service", store, exporteddomain.NewMemoryStore())
	if _, err := transfer.ExportWorkspacePath(context.Background(), &sandboxv1.ExportWorkspacePathRequest{ServiceWorkspaceId: workspace.ID, Path: "file.txt"}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}
}

func newTestTransferService(t *testing.T) (*WorkspaceTransferService, exporteddomain.Store, workspacedomain.Workspace) {
	store := workspacedomain.NewMemoryStore()
	exportStore := exporteddomain.NewMemoryStore()
	root := t.TempDir()
	workspace := workspacedomain.Workspace{
		ID:               "ws-test",
		SandboxProfileID: "local-process",
		Status:           workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
		RootPath:         root,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	created, err := store.Create(context.Background(), workspace)
	if err != nil {
		t.Fatalf("Create workspace returned error: %v", err)
	}
	return NewWorkspaceTransferService("sandbox-service", store, exportStore), exportStore, created
}
