package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	workspacestore "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWorkspaceViewServiceListWorkspaceDir(t *testing.T) {
	view, backing, workspace := newTestViewService(t)
	backing.listResp = &workspacestore.DirListing{
		Directory: workspacestore.PathInfo{Path: "", Name: "", Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_DIRECTORY},
		Entries: []workspacestore.PathInfo{
			{Path: "a.txt", Name: "a.txt", Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE, SizeBytes: 3, ModifiedAt: time.Now().UTC()},
			{Path: "src", Name: "src", Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_DIRECTORY},
		},
		NextToken: "2",
	}

	resp, err := view.ListWorkspaceDir(context.Background(), &sandboxv1.ListWorkspaceDirRequest{
		ServiceWorkspaceId: workspace.ID,
		Path:               "",
		PageSize:           2,
	})
	if err != nil {
		t.Fatalf("ListWorkspaceDir returned error: %v", err)
	}
	if backing.lastList.WorkspaceID != workspace.StoreWorkspaceID || backing.lastList.PageSize != 2 {
		t.Fatalf("unexpected list request: %+v", backing.lastList)
	}
	if resp.GetDirectory().GetPath() != "" || resp.GetDirectory().GetKind() != sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_DIRECTORY {
		t.Fatalf("unexpected directory ref: %+v", resp.GetDirectory())
	}
	if got := entryNames(resp.GetEntries()); strings.Join(got, ",") != "a.txt,src" {
		t.Fatalf("unexpected entries: %v", got)
	}
	if resp.GetNextPageToken() != "2" {
		t.Fatalf("unexpected next page token: %q", resp.GetNextPageToken())
	}
	assertWorkspacePathRef(t, resp.GetEntries()[0].GetRef(), workspace, "a.txt")
}

func TestWorkspaceViewServicePreviewWorkspaceFile(t *testing.T) {
	view, backing, workspace := newTestViewService(t)
	modifiedAt := time.Now().UTC()
	backing.previewResp = &workspacestore.FilePreview{
		File:      workspacestore.PathInfo{Path: "report.txt", Name: "report.txt", Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE, SizeBytes: 11, ModifiedAt: modifiedAt},
		MimeType:  "text/plain; charset=utf-8",
		Bytes:     []byte("hello"),
		Truncated: true,
	}

	resp, err := view.PreviewWorkspaceFile(context.Background(), &sandboxv1.PreviewWorkspaceFileRequest{
		ServiceWorkspaceId: workspace.ID,
		Path:               "report.txt",
		MaxBytes:           5,
	})
	if err != nil {
		t.Fatalf("PreviewWorkspaceFile returned error: %v", err)
	}
	if backing.lastPreview.WorkspaceID != workspace.StoreWorkspaceID || backing.lastPreview.Path != "report.txt" || backing.lastPreview.MaxBytes != 5 {
		t.Fatalf("unexpected preview request: %+v", backing.lastPreview)
	}
	assertWorkspacePathRef(t, resp.GetFile(), workspace, "report.txt")
	if string(resp.GetPreviewBytes()) != "hello" || !resp.GetTruncated() || resp.GetSizeBytes() != 11 || resp.GetMimeType() == "" || resp.GetModifiedAt() == nil {
		t.Fatalf("unexpected preview response: %+v", resp)
	}
}

func TestWorkspaceViewServiceMissingWorkspace(t *testing.T) {
	view, _, _ := newTestViewService(t)
	_, err := view.ListWorkspaceDir(context.Background(), &sandboxv1.ListWorkspaceDirRequest{ServiceWorkspaceId: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestWorkspaceViewServiceMapsStoreErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "invalid path", err: workspacestore.ErrInvalidPath, code: codes.InvalidArgument},
		{name: "not found", err: workspacestore.ErrPathNotFound, code: codes.NotFound},
		{name: "not directory", err: workspacestore.ErrPathNotDirectory, code: codes.FailedPrecondition},
		{name: "is directory", err: workspacestore.ErrPathIsDirectory, code: codes.FailedPrecondition},
		{name: "symlink", err: workspacestore.ErrSymlinkNotAllowed, code: codes.PermissionDenied},
		{name: "escape", err: workspacestore.ErrPathEscapesWorkspace, code: codes.PermissionDenied},
		{name: "not ready", err: workspacestore.ErrWorkspaceNotReady, code: codes.FailedPrecondition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view, backing, workspace := newTestViewService(t)
			backing.listErr = tt.err
			_, err := view.ListWorkspaceDir(context.Background(), &sandboxv1.ListWorkspaceDirRequest{ServiceWorkspaceId: workspace.ID})
			if status.Code(err) != tt.code {
				t.Fatalf("expected %s, got %v", tt.code, err)
			}
		})
	}
}

func newTestViewService(t *testing.T) (*WorkspaceViewService, *fakeBackingStore, workspacedomain.Workspace) {
	t.Helper()
	store := workspacedomain.NewMemoryStore()
	backing := &fakeBackingStore{}
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
	return NewWorkspaceViewService("sandbox-service", store, backing), backing, created
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

type fakeBackingStore struct {
	createResp  *workspacestore.BackingWorkspace
	createErr   error
	listResp    *workspacestore.DirListing
	listErr     error
	previewResp *workspacestore.FilePreview
	previewErr  error
	exportResp  *workspacestore.ExportedPath
	exportErr   error
	importResp  *workspacestore.ImportedFile
	importErr   error

	lastCreate        workspacestore.CreateBackingWorkspaceRequest
	lastList          workspacestore.ListDirRequest
	lastPreview       workspacestore.PreviewFileRequest
	lastExport        workspacestore.ExportPathRequest
	lastImport        workspacestore.ImportFileRequest
	lastImportContent string
}

func (f *fakeBackingStore) Kind() string { return "fake" }

func (f *fakeBackingStore) CreateBackingWorkspace(_ context.Context, req workspacestore.CreateBackingWorkspaceRequest) (*workspacestore.BackingWorkspace, error) {
	f.lastCreate = req
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createResp != nil {
		return f.createResp, nil
	}
	return &workspacestore.BackingWorkspace{
		ID:               req.WorkspaceID,
		StoreKind:        "fake",
		StoreWorkspaceID: req.WorkspaceID,
		SandboxProfileID: req.SandboxProfileID,
		Status:           workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
	}, nil
}

func (f *fakeBackingStore) DeleteBackingWorkspace(context.Context, string) error { return nil }

func (f *fakeBackingStore) ListDir(_ context.Context, req workspacestore.ListDirRequest) (*workspacestore.DirListing, error) {
	f.lastList = req
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.listResp != nil {
		return f.listResp, nil
	}
	return &workspacestore.DirListing{
		Directory: workspacestore.PathInfo{Path: req.Path, Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_DIRECTORY},
	}, nil
}

func (f *fakeBackingStore) PreviewFile(_ context.Context, req workspacestore.PreviewFileRequest) (*workspacestore.FilePreview, error) {
	f.lastPreview = req
	if f.previewErr != nil {
		return nil, f.previewErr
	}
	if f.previewResp != nil {
		return f.previewResp, nil
	}
	return &workspacestore.FilePreview{
		File:     workspacestore.PathInfo{Path: req.Path, Name: req.Path, Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE},
		MimeType: "text/plain",
		Bytes:    []byte("preview"),
	}, nil
}

func (f *fakeBackingStore) StatPath(context.Context, workspacestore.StatPathRequest) (*workspacestore.PathInfo, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeBackingStore) ExportPath(_ context.Context, req workspacestore.ExportPathRequest) (*workspacestore.ExportedPath, error) {
	f.lastExport = req
	if f.exportErr != nil {
		return nil, f.exportErr
	}
	if f.exportResp != nil {
		return f.exportResp, nil
	}
	return &workspacestore.ExportedPath{
		Source:    workspacestore.PathInfo{Path: req.Path, Name: req.Path, Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE, SizeBytes: 12},
		MimeType:  "text/plain",
		SizeBytes: 12,
		Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("exported")), nil
		},
	}, nil
}

func (f *fakeBackingStore) ImportFile(_ context.Context, req workspacestore.ImportFileRequest) (*workspacestore.ImportedFile, error) {
	f.lastImport = req
	if req.Source != nil {
		body, err := io.ReadAll(req.Source)
		if err != nil {
			return nil, err
		}
		f.lastImportContent = string(body)
	}
	if f.importErr != nil {
		return nil, f.importErr
	}
	if f.importResp != nil {
		return f.importResp, nil
	}
	size := int64(len(f.lastImportContent))
	return &workspacestore.ImportedFile{
		Path:        workspacestore.PathInfo{Path: req.Path, Name: req.Path, Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE, SizeBytes: size},
		MimeType:    req.MimeType,
		SizeBytes:   size,
		ContentHash: req.ExpectedHash,
	}, nil
}
