package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	exporteddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/exportedresource"
	resourceblob "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/resourceblob"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	workspacestore "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWorkspaceTransferServiceExportWorkspacePath(t *testing.T) {
	transfer, backing, blobStore, exportStore, workspace := newTestTransferService(t)
	backing.exportResp = &workspacestore.ExportedPath{
		Source:    workspacestore.PathInfo{Path: "outputs/report.txt", Name: "report.txt", Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE, SizeBytes: 6},
		MimeType:  "text/plain; charset=utf-8",
		SizeBytes: 6,
		Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("report")), nil
		},
	}
	blobStore.contentHash = "sha256:abc"

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
	if blobStore.lastPut.ResourceID != resp.GetResource().GetId() || blobStore.lastPut.Name != "custom.txt" || blobStore.lastPut.MimeType != "text/plain; charset=utf-8" {
		t.Fatalf("unexpected blob put request: %+v", blobStore.lastPut)
	}
	if blobStore.lastPutContent != "report" {
		t.Fatalf("unexpected blob content: %q", blobStore.lastPutContent)
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
	if resp.GetResource().GetContentHash() != "sha256:abc" {
		t.Fatalf("unexpected content hash: %q", resp.GetResource().GetContentHash())
	}
	record, err := exportStore.Get(context.Background(), resp.GetResource().GetId())
	if err != nil {
		t.Fatalf("export store Get returned error: %v", err)
	}
	if record.SourceServiceWorkspaceID != workspace.ID ||
		record.SourceWorkspacePath != "outputs/report.txt" ||
		record.BlobStoreKind != "fakeblob" ||
		record.BlobID != resp.GetResource().GetId() ||
		record.ContentHash != "sha256:abc" ||
		record.SizeBytes != 6 {
		t.Fatalf("unexpected export record: %+v", record)
	}
}

func TestWorkspaceTransferServiceExportWorkspacePathDefaultNameAndMime(t *testing.T) {
	transfer, backing, _, _, workspace := newTestTransferService(t)
	backing.exportResp = &workspacestore.ExportedPath{
		Source:    workspacestore.PathInfo{Path: "outputs/blob.unknown", Name: "blob.unknown", Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE, SizeBytes: 4},
		MimeType:  "application/octet-stream",
		SizeBytes: 4,
		Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("blob")), nil
		},
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
			transfer, backing, _, _, workspace := newTestTransferService(t)
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
	transfer, _, _, _, _ := newTestTransferService(t)
	_, err := transfer.ExportWorkspacePath(context.Background(), &sandboxv1.ExportWorkspacePathRequest{ServiceWorkspaceId: "missing", Path: "file.txt"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestWorkspaceTransferServiceExportWorkspacePathBlobPutError(t *testing.T) {
	transfer, backing, blobStore, _, workspace := newTestTransferService(t)
	backing.exportResp = &workspacestore.ExportedPath{
		Source:    workspacestore.PathInfo{Path: "report.txt", Name: "report.txt", Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE},
		MimeType:  "text/plain",
		SizeBytes: 6,
		Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("report")), nil
		},
	}
	blobStore.putErr = resourceblob.ErrStoreNotReady

	_, err := transfer.ExportWorkspacePath(context.Background(), &sandboxv1.ExportWorkspacePathRequest{ServiceWorkspaceId: workspace.ID, Path: "report.txt"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}
}

func TestWorkspaceTransferServiceExportWorkspacePathOpenError(t *testing.T) {
	transfer, backing, _, _, workspace := newTestTransferService(t)
	backing.exportResp = &workspacestore.ExportedPath{
		Source:    workspacestore.PathInfo{Path: "report.txt", Name: "report.txt", Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE},
		MimeType:  "text/plain",
		SizeBytes: 6,
		Open: func(context.Context) (io.ReadCloser, error) {
			return nil, errors.New("open failed")
		},
	}

	_, err := transfer.ExportWorkspacePath(context.Background(), &sandboxv1.ExportWorkspacePathRequest{ServiceWorkspaceId: workspace.ID, Path: "report.txt"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
}

func TestWorkspaceTransferServiceRollsBackBlobOnExportRecordFailure(t *testing.T) {
	transfer, backing, blobStore, _, workspace := newTestTransferService(t)
	backing.exportResp = &workspacestore.ExportedPath{
		Source:    workspacestore.PathInfo{Path: "report.txt", Name: "report.txt", Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE},
		MimeType:  "text/plain",
		SizeBytes: 6,
		Open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("report")), nil
		},
	}
	transfer.exportStore = failingExportStore{}

	_, err := transfer.ExportWorkspacePath(context.Background(), &sandboxv1.ExportWorkspacePathRequest{ServiceWorkspaceId: workspace.ID, Path: "report.txt"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
	if !blobStore.deleted {
		t.Fatal("expected blob rollback delete")
	}
}

func TestWorkspaceTransferServiceImportResourceToWorkspace(t *testing.T) {
	transfer, backing, _, _, workspace := newTestTransferService(t)
	stream := &fakeImportResourceServer{
		ctx: context.Background(),
		messages: []*sandboxv1.ImportResourceToWorkspaceRequest{
			{Payload: &sandboxv1.ImportResourceToWorkspaceRequest_Header{Header: &sandboxv1.ImportResourceToWorkspaceHeader{
				ServiceWorkspaceId: workspace.ID,
				Resource: &resourcev1.ResourceRef{
					Id:          "res_1",
					Name:        "report.txt",
					MimeType:    "text/plain",
					SizeBytes:   11,
					ContentHash: "sha256:abc",
				},
				DestinationPath: "inputs/report.txt",
				ConflictPolicy:  sandboxv1.ImportConflictPolicy_IMPORT_CONFLICT_POLICY_OVERWRITE,
			}}},
			{Payload: &sandboxv1.ImportResourceToWorkspaceRequest_Data{Data: []byte("hello ")}},
			{Payload: &sandboxv1.ImportResourceToWorkspaceRequest_Data{Data: []byte("world")}},
		},
	}
	backing.importResp = &workspacestore.ImportedFile{
		Path:        workspacestore.PathInfo{Path: "inputs/report.txt", Name: "report.txt", Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE},
		MimeType:    "text/plain",
		SizeBytes:   11,
		ContentHash: "sha256:abc",
	}

	err := transfer.ImportResourceToWorkspace(stream)
	if err != nil {
		t.Fatalf("ImportResourceToWorkspace returned error: %v", err)
	}
	if backing.lastImport.WorkspaceID != workspace.StoreWorkspaceID ||
		backing.lastImport.Path != "inputs/report.txt" ||
		backing.lastImport.MimeType != "text/plain" ||
		backing.lastImport.ExpectedSizeBytes != 11 ||
		backing.lastImport.ExpectedHash != "sha256:abc" ||
		backing.lastImport.ConflictPolicy != workspacestore.ImportConflictOverwrite {
		t.Fatalf("unexpected import request: %+v", backing.lastImport)
	}
	if backing.lastImportContent != "hello world" {
		t.Fatalf("unexpected streamed import content: %q", backing.lastImportContent)
	}
	resp := stream.response
	if resp.GetPath().GetPath() != "inputs/report.txt" ||
		resp.GetPath().GetWorkspace().GetServiceWorkspaceId() != workspace.ID ||
		resp.GetSizeBytes() != 11 ||
		resp.GetContentHash() != "sha256:abc" ||
		resp.GetMimeType() != "text/plain" {
		t.Fatalf("unexpected import response: %+v", resp)
	}
}

func TestWorkspaceTransferServiceImportResourceToWorkspaceErrors(t *testing.T) {
	tests := []struct {
		name     string
		messages []*sandboxv1.ImportResourceToWorkspaceRequest
		code     codes.Code
	}{
		{name: "empty stream", code: codes.InvalidArgument},
		{
			name: "data before header",
			messages: []*sandboxv1.ImportResourceToWorkspaceRequest{
				{Payload: &sandboxv1.ImportResourceToWorkspaceRequest_Data{Data: []byte("x")}},
			},
			code: codes.InvalidArgument,
		},
		{
			name: "missing resource id",
			messages: []*sandboxv1.ImportResourceToWorkspaceRequest{
				{Payload: &sandboxv1.ImportResourceToWorkspaceRequest_Header{Header: &sandboxv1.ImportResourceToWorkspaceHeader{ServiceWorkspaceId: "ws-test", Resource: &resourcev1.ResourceRef{}, DestinationPath: "file.txt"}}},
			},
			code: codes.InvalidArgument,
		},
		{
			name: "missing destination",
			messages: []*sandboxv1.ImportResourceToWorkspaceRequest{
				{Payload: &sandboxv1.ImportResourceToWorkspaceRequest_Header{Header: &sandboxv1.ImportResourceToWorkspaceHeader{ServiceWorkspaceId: "ws-test", Resource: &resourcev1.ResourceRef{Id: "res_1"}}}},
			},
			code: codes.InvalidArgument,
		},
		{
			name: "missing workspace",
			messages: []*sandboxv1.ImportResourceToWorkspaceRequest{
				{Payload: &sandboxv1.ImportResourceToWorkspaceRequest_Header{Header: &sandboxv1.ImportResourceToWorkspaceHeader{ServiceWorkspaceId: "missing", Resource: &resourcev1.ResourceRef{Id: "res_1"}, DestinationPath: "file.txt"}}},
			},
			code: codes.NotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transfer, _, _, _, _ := newTestTransferService(t)
			err := transfer.ImportResourceToWorkspace(&fakeImportResourceServer{ctx: context.Background(), messages: tt.messages})
			if status.Code(err) != tt.code {
				t.Fatalf("expected %s, got %v", tt.code, err)
			}
		})
	}
}

func TestWorkspaceTransferServiceImportResourceToWorkspaceStoreError(t *testing.T) {
	transfer, backing, _, _, workspace := newTestTransferService(t)
	backing.importErr = workspacestore.ErrPathAlreadyExists
	stream := &fakeImportResourceServer{
		ctx: context.Background(),
		messages: []*sandboxv1.ImportResourceToWorkspaceRequest{
			{Payload: &sandboxv1.ImportResourceToWorkspaceRequest_Header{Header: &sandboxv1.ImportResourceToWorkspaceHeader{
				ServiceWorkspaceId: workspace.ID,
				Resource:           &resourcev1.ResourceRef{Id: "res_1"},
				DestinationPath:    "file.txt",
			}}},
		},
	}
	err := transfer.ImportResourceToWorkspace(stream)
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v", err)
	}
}

func TestWorkspaceTransferServiceImportResourceToWorkspaceRejectsRepeatedHeader(t *testing.T) {
	transfer, _, _, _, workspace := newTestTransferService(t)
	stream := &fakeImportResourceServer{
		ctx: context.Background(),
		messages: []*sandboxv1.ImportResourceToWorkspaceRequest{
			{Payload: &sandboxv1.ImportResourceToWorkspaceRequest_Header{Header: &sandboxv1.ImportResourceToWorkspaceHeader{
				ServiceWorkspaceId: workspace.ID,
				Resource:           &resourcev1.ResourceRef{Id: "res_1"},
				DestinationPath:    "file.txt",
			}}},
			{Payload: &sandboxv1.ImportResourceToWorkspaceRequest_Header{Header: &sandboxv1.ImportResourceToWorkspaceHeader{
				ServiceWorkspaceId: workspace.ID,
				Resource:           &resourcev1.ResourceRef{Id: "res_1"},
				DestinationPath:    "file.txt",
			}}},
		},
	}
	err := transfer.ImportResourceToWorkspace(stream)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func newTestTransferService(t *testing.T) (*WorkspaceTransferService, *fakeBackingStore, *fakeBlobStore, exporteddomain.Store, workspacedomain.Workspace) {
	t.Helper()
	store := workspacedomain.NewMemoryStore()
	backing := &fakeBackingStore{}
	blobStore := &fakeBlobStore{contentHash: "sha256:fake"}
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
	return NewWorkspaceTransferService("sandbox-service", store, backing, blobStore, exportStore), backing, blobStore, exportStore, created
}

type fakeBlobStore struct {
	putErr      error
	contentHash string
	deleted     bool

	lastPut        resourceblob.PutRequest
	lastPutContent string
}

func (f *fakeBlobStore) Kind() string { return "fakeblob" }

func (f *fakeBlobStore) Put(_ context.Context, req resourceblob.PutRequest) (*resourceblob.StoredBlob, error) {
	f.lastPut = req
	if req.Source != nil {
		body, _ := io.ReadAll(req.Source)
		f.lastPutContent = string(body)
	}
	if f.putErr != nil {
		return nil, f.putErr
	}
	size := int64(len(f.lastPutContent))
	return &resourceblob.StoredBlob{
		ResourceID:   req.ResourceID,
		StoreKind:    f.Kind(),
		StoreBlobID:  req.ResourceID,
		Name:         req.Name,
		MimeType:     req.MimeType,
		SizeBytes:    size,
		ContentHash:  f.contentHash,
		MetadataJSON: append([]byte(nil), req.MetadataJSON...),
	}, nil
}

func (f *fakeBlobStore) Open(context.Context, string) (io.ReadCloser, *resourceblob.BlobInfo, error) {
	return nil, nil, errors.New("not implemented")
}

func (f *fakeBlobStore) Stat(context.Context, string) (*resourceblob.BlobInfo, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeBlobStore) Delete(context.Context, string) error {
	f.deleted = true
	return nil
}

type failingExportStore struct{}

func (failingExportStore) Create(context.Context, exporteddomain.Record) (exporteddomain.Record, error) {
	return exporteddomain.Record{}, errors.New("create failed")
}

func (failingExportStore) Get(context.Context, string) (exporteddomain.Record, error) {
	return exporteddomain.Record{}, errors.New("not implemented")
}

type fakeImportResourceServer struct {
	sandboxv1.WorkspaceTransferService_ImportResourceToWorkspaceServer
	ctx      context.Context
	messages []*sandboxv1.ImportResourceToWorkspaceRequest
	response *sandboxv1.ImportResourceToWorkspaceResponse
}

func (s *fakeImportResourceServer) Context() context.Context {
	return s.ctx
}

func (s *fakeImportResourceServer) Recv() (*sandboxv1.ImportResourceToWorkspaceRequest, error) {
	if len(s.messages) == 0 {
		return nil, io.EOF
	}
	msg := s.messages[0]
	s.messages = s.messages[1:]
	return msg, nil
}

func (s *fakeImportResourceServer) SendAndClose(resp *sandboxv1.ImportResourceToWorkspaceResponse) error {
	s.response = resp
	return nil
}
