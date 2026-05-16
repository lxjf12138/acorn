package service

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWorkspaceViewServiceListWorkspaceDir(t *testing.T) {
	view, workspace := newTestViewService(t)
	writeFile(t, workspace.RootPath, "b.txt", "bee")
	writeFile(t, workspace.RootPath, "a.txt", "aye")
	if err := os.Mkdir(filepath.Join(workspace.RootPath, "src"), 0o700); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}

	resp, err := view.ListWorkspaceDir(context.Background(), &sandboxv1.ListWorkspaceDirRequest{
		ServiceWorkspaceId: workspace.ID,
		Path:               "",
		PageSize:           2,
	})
	if err != nil {
		t.Fatalf("ListWorkspaceDir returned error: %v", err)
	}
	if resp.GetDirectory().GetPath() != "" || resp.GetDirectory().GetKind() != sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_DIRECTORY {
		t.Fatalf("unexpected directory ref: %+v", resp.GetDirectory())
	}
	if got := entryNames(resp.GetEntries()); strings.Join(got, ",") != "a.txt,b.txt" {
		t.Fatalf("unexpected entries: %v", got)
	}
	if resp.GetNextPageToken() == "" {
		t.Fatal("expected next page token")
	}
	for _, entry := range resp.GetEntries() {
		assertWorkspacePathRef(t, entry.GetRef(), workspace, entry.GetName())
	}
}

func TestWorkspaceViewServiceListWorkspaceDirPagination(t *testing.T) {
	view, workspace := newTestViewService(t)
	writeFile(t, workspace.RootPath, "a.txt", "a")
	writeFile(t, workspace.RootPath, "b.txt", "b")
	writeFile(t, workspace.RootPath, "c.txt", "c")

	first, err := view.ListWorkspaceDir(context.Background(), &sandboxv1.ListWorkspaceDirRequest{
		ServiceWorkspaceId: workspace.ID,
		PageSize:           2,
	})
	if err != nil {
		t.Fatalf("ListWorkspaceDir returned error: %v", err)
	}
	second, err := view.ListWorkspaceDir(context.Background(), &sandboxv1.ListWorkspaceDirRequest{
		ServiceWorkspaceId: workspace.ID,
		PageSize:           2,
		PageToken:          first.GetNextPageToken(),
	})
	if err != nil {
		t.Fatalf("ListWorkspaceDir returned error: %v", err)
	}
	if got := entryNames(second.GetEntries()); strings.Join(got, ",") != "c.txt" {
		t.Fatalf("unexpected second page: %v", got)
	}
	if second.GetNextPageToken() != "" {
		t.Fatalf("unexpected next page token: %q", second.GetNextPageToken())
	}
}

func TestWorkspaceViewServiceListWorkspaceDirErrors(t *testing.T) {
	view, workspace := newTestViewService(t)
	writeFile(t, workspace.RootPath, "file.txt", "content")
	tests := []struct {
		name string
		req  *sandboxv1.ListWorkspaceDirRequest
		code codes.Code
	}{
		{name: "missing workspace", req: &sandboxv1.ListWorkspaceDirRequest{ServiceWorkspaceId: "missing"}, code: codes.NotFound},
		{name: "file path", req: &sandboxv1.ListWorkspaceDirRequest{ServiceWorkspaceId: workspace.ID, Path: "file.txt"}, code: codes.FailedPrecondition},
		{name: "traversal", req: &sandboxv1.ListWorkspaceDirRequest{ServiceWorkspaceId: workspace.ID, Path: "../secret"}, code: codes.InvalidArgument},
		{name: "bad page token", req: &sandboxv1.ListWorkspaceDirRequest{ServiceWorkspaceId: workspace.ID, PageToken: "bad"}, code: codes.InvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := view.ListWorkspaceDir(context.Background(), tt.req)
			if status.Code(err) != tt.code {
				t.Fatalf("expected %s, got %v", tt.code, err)
			}
		})
	}
}

func TestWorkspaceViewServicePreviewWorkspaceFile(t *testing.T) {
	view, workspace := newTestViewService(t)
	writeFile(t, workspace.RootPath, "report.txt", "hello world")

	resp, err := view.PreviewWorkspaceFile(context.Background(), &sandboxv1.PreviewWorkspaceFileRequest{
		ServiceWorkspaceId: workspace.ID,
		Path:               "report.txt",
		MaxBytes:           5,
	})
	if err != nil {
		t.Fatalf("PreviewWorkspaceFile returned error: %v", err)
	}
	assertWorkspacePathRef(t, resp.GetFile(), workspace, "report.txt")
	if string(resp.GetPreviewBytes()) != "hello" {
		t.Fatalf("unexpected preview bytes: %q", string(resp.GetPreviewBytes()))
	}
	if !resp.GetTruncated() {
		t.Fatal("expected truncated preview")
	}
	if resp.GetSizeBytes() != int64(len("hello world")) {
		t.Fatalf("unexpected size: %d", resp.GetSizeBytes())
	}
	if resp.GetMimeType() == "" || resp.GetModifiedAt() == nil {
		t.Fatalf("missing mime type or modified_at: %+v", resp)
	}
}

func TestWorkspaceViewServicePreviewWorkspaceFileErrors(t *testing.T) {
	view, workspace := newTestViewService(t)
	if err := os.Mkdir(filepath.Join(workspace.RootPath, "dir"), 0o700); err != nil {
		t.Fatalf("mkdir dir: %v", err)
	}
	writeFile(t, workspace.RootPath, "file.txt", "content")
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(workspace.RootPath, "file.txt"), filepath.Join(workspace.RootPath, "link.txt")); err != nil {
			t.Fatalf("symlink: %v", err)
		}
	}

	tests := []struct {
		name string
		req  *sandboxv1.PreviewWorkspaceFileRequest
		code codes.Code
	}{
		{name: "missing file", req: &sandboxv1.PreviewWorkspaceFileRequest{ServiceWorkspaceId: workspace.ID, Path: "missing.txt"}, code: codes.NotFound},
		{name: "directory", req: &sandboxv1.PreviewWorkspaceFileRequest{ServiceWorkspaceId: workspace.ID, Path: "dir"}, code: codes.FailedPrecondition},
		{name: "traversal", req: &sandboxv1.PreviewWorkspaceFileRequest{ServiceWorkspaceId: workspace.ID, Path: "../secret"}, code: codes.InvalidArgument},
	}
	if runtime.GOOS != "windows" {
		tests = append(tests, struct {
			name string
			req  *sandboxv1.PreviewWorkspaceFileRequest
			code codes.Code
		}{name: "symlink", req: &sandboxv1.PreviewWorkspaceFileRequest{ServiceWorkspaceId: workspace.ID, Path: "link.txt"}, code: codes.PermissionDenied})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := view.PreviewWorkspaceFile(context.Background(), tt.req)
			if status.Code(err) != tt.code {
				t.Fatalf("expected %s, got %v", tt.code, err)
			}
		})
	}
}

func newTestViewService(t *testing.T) (*WorkspaceViewService, workspacedomain.Workspace) {
	store := workspacedomain.NewMemoryStore()
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
	return NewWorkspaceViewService("sandbox-service", store), created
}

func writeFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func entryNames(entries []*sandboxv1.WorkspaceDirEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.GetName())
	}
	return out
}

func assertWorkspacePathRef(t *testing.T, ref *sandboxv1.WorkspacePathRef, workspace workspacedomain.Workspace, path string) {
	t.Helper()
	if ref.GetWorkspace().GetServiceId() != "sandbox-service" ||
		ref.GetWorkspace().GetServiceWorkspaceId() != workspace.ID ||
		ref.GetWorkspace().GetSandboxProfileId() != workspace.SandboxProfileID ||
		ref.GetPath() != path {
		t.Fatalf("unexpected workspace path ref: %+v", ref)
	}
}
