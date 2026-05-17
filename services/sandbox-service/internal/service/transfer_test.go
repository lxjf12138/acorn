package service

import (
	"context"
	"testing"
	"time"

	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	exporteddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/exportedresource"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	workspacestore "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWorkspaceTransferServiceExportWorkspacePath(t *testing.T) {
	transfer, backing, exportStore, workspace := newTestTransferService(t)
	backing.exportResp = &workspacestore.ExportedPath{
		Source:    workspacestore.PathInfo{Path: "outputs/report.txt", Name: "report.txt", Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE, SizeBytes: 6},
		MimeType:  "text/plain; charset=utf-8",
		SizeBytes: 6,
	}

	resp, err := transfer.ExportWorkspacePath(context.Background(), &sandboxv1.ExportWorkspacePathRequest{
		ServiceWorkspaceId: workspace.ID,
		Path:               "outputs/report.txt",
		ResourceName:       "custom.txt",
	})
	if err != nil {
		t.Fatalf("ExportWorkspacePath returned error: %v", err)
	}
	if backing.lastExport.WorkspaceID != workspace.StoreWorkspaceID || backing.lastExport.Path != "outputs/report.txt" {
		t.Fatalf("unexpected export request: %+v", backing.lastExport)
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
	if resp.GetResource().GetSizeBytes() != 6 {
		t.Fatalf("unexpected resource size: %d", resp.GetResource().GetSizeBytes())
	}
	record, err := exportStore.Get(context.Background(), resp.GetResource().GetId())
	if err != nil {
		t.Fatalf("export store Get returned error: %v", err)
	}
	if record.ServiceWorkspaceID != workspace.ID || record.WorkspacePath != "outputs/report.txt" || record.SizeBytes != 6 {
		t.Fatalf("unexpected export record: %+v", record)
	}
}

func TestWorkspaceTransferServiceExportWorkspacePathDefaultNameAndMime(t *testing.T) {
	transfer, backing, _, workspace := newTestTransferService(t)
	backing.exportResp = &workspacestore.ExportedPath{
		Source:    workspacestore.PathInfo{Path: "outputs/blob.unknown", Name: "blob.unknown", Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE, SizeBytes: 4},
		MimeType:  "application/octet-stream",
		SizeBytes: 4,
	}

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
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "invalid path", err: workspacestore.ErrInvalidPath, code: codes.InvalidArgument},
		{name: "missing path", err: workspacestore.ErrPathNotFound, code: codes.NotFound},
		{name: "directory", err: workspacestore.ErrPathIsDirectory, code: codes.FailedPrecondition},
		{name: "symlink", err: workspacestore.ErrSymlinkNotAllowed, code: codes.PermissionDenied},
		{name: "escape", err: workspacestore.ErrPathEscapesWorkspace, code: codes.PermissionDenied},
		{name: "not ready", err: workspacestore.ErrWorkspaceNotReady, code: codes.FailedPrecondition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transfer, backing, _, workspace := newTestTransferService(t)
			backing.exportErr = tt.err
			_, err := transfer.ExportWorkspacePath(context.Background(), &sandboxv1.ExportWorkspacePathRequest{
				ServiceWorkspaceId: workspace.ID,
				Path:               "file.txt",
			})
			if status.Code(err) != tt.code {
				t.Fatalf("expected %s, got %v", tt.code, err)
			}
		})
	}
}

func TestWorkspaceTransferServiceExportWorkspacePathMissingWorkspace(t *testing.T) {
	transfer, _, _, _ := newTestTransferService(t)
	_, err := transfer.ExportWorkspacePath(context.Background(), &sandboxv1.ExportWorkspacePathRequest{ServiceWorkspaceId: "missing", Path: "file.txt"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func newTestTransferService(t *testing.T) (*WorkspaceTransferService, *fakeBackingStore, exporteddomain.Store, workspacedomain.Workspace) {
	t.Helper()
	store := workspacedomain.NewMemoryStore()
	backing := &fakeBackingStore{}
	exportStore := exporteddomain.NewMemoryStore()
	workspace := workspacedomain.Workspace{
		ID:               "ws-test",
		SandboxProfileID: "local-process",
		Status:           workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
		StoreKind:        "fake",
		StoreWorkspaceID: "ws-backing",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	created, err := store.Create(context.Background(), workspace)
	if err != nil {
		t.Fatalf("Create workspace returned error: %v", err)
	}
	return NewWorkspaceTransferService("sandbox-service", store, backing, exportStore), backing, exportStore, created
}
