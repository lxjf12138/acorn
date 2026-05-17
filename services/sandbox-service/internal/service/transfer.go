package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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

type WorkspaceTransferService struct {
	sandboxv1.UnimplementedWorkspaceTransferServiceServer

	serviceID      string
	workspaceStore workspacedomain.Store
	backing        workspacestore.Store
	blobStore      resourceblob.Store
	exportStore    exporteddomain.Store
}

func NewWorkspaceTransferService(serviceID string, workspaceStore workspacedomain.Store, backing workspacestore.Store, blobStore resourceblob.Store, exportStore exporteddomain.Store) *WorkspaceTransferService {
	return &WorkspaceTransferService{
		serviceID:      serviceID,
		workspaceStore: workspaceStore,
		backing:        backing,
		blobStore:      blobStore,
		exportStore:    exportStore,
	}
}

func (s *WorkspaceTransferService) ImportResourceToWorkspace(stream sandboxv1.WorkspaceTransferService_ImportResourceToWorkspaceServer) error {
	first, err := stream.Recv()
	if err != nil {
		if err == io.EOF {
			return status.Error(codes.InvalidArgument, "import header is required")
		}
		return err
	}
	header := first.GetHeader()
	if header == nil {
		return status.Error(codes.InvalidArgument, "first import message must be header")
	}
	if header.GetServiceWorkspaceId() == "" {
		return status.Error(codes.InvalidArgument, "service_workspace_id is required")
	}
	if header.GetResource().GetId() == "" {
		return status.Error(codes.InvalidArgument, "resource.id is required")
	}
	if header.GetDestinationPath() == "" {
		return status.Error(codes.InvalidArgument, "destination_path is required")
	}
	workspace, err := s.workspaceStore.Get(stream.Context(), header.GetServiceWorkspaceId())
	if errors.Is(err, workspacedomain.ErrNotFound) {
		return status.Error(codes.NotFound, "hosted workspace not found")
	}
	if err != nil {
		return status.Errorf(codes.Internal, "get hosted workspace: %v", err)
	}
	if workspace.StoreWorkspaceID == "" {
		workspace.StoreWorkspaceID = workspace.ID
	}

	imported, err := s.backing.ImportFile(stream.Context(), workspacestore.ImportFileRequest{
		WorkspaceID:       workspace.StoreWorkspaceID,
		Path:              header.GetDestinationPath(),
		Name:              header.GetResource().GetName(),
		MimeType:          header.GetResource().GetMimeType(),
		Source:            &importResourceStreamReader{stream: stream},
		ExpectedSizeBytes: header.GetResource().GetSizeBytes(),
		ExpectedHash:      header.GetResource().GetContentHash(),
		ConflictPolicy:    importConflictPolicy(header.GetConflictPolicy()),
	})
	if err != nil {
		if _, ok := status.FromError(err); ok {
			return err
		}
		return mapWorkspaceStoreError(err)
	}
	return stream.SendAndClose(&sandboxv1.ImportResourceToWorkspaceResponse{
		Path: &sandboxv1.WorkspacePathRef{
			Workspace: &workspacev1.WorkspaceHostRef{
				ServiceId:          s.serviceID,
				ServiceWorkspaceId: workspace.ID,
				SandboxProfileId:   workspace.SandboxProfileID,
			},
			Path:        imported.Path.Path,
			Kind:        imported.Path.Kind,
			DisplayName: imported.Path.Name,
		},
		SizeBytes:   imported.SizeBytes,
		ContentHash: imported.ContentHash,
		MimeType:    imported.MimeType,
	})
}

func (s *WorkspaceTransferService) ExportWorkspacePath(ctx context.Context, req *sandboxv1.ExportWorkspacePathRequest) (*sandboxv1.ExportWorkspacePathResponse, error) {
	workspace, exported, err := s.exportableFile(ctx, req.GetServiceWorkspaceId(), req.GetPath())
	if err != nil {
		return nil, err
	}

	resourceID := newExportedResourceID()
	name := req.GetResourceName()
	if name == "" {
		name = exported.Source.Name
	}
	mimeType := req.GetMimeType()
	if mimeType == "" {
		mimeType = exported.MimeType
	}

	reader, err := exported.Open(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open exported workspace path: %v", err)
	}
	defer reader.Close()

	blob, err := s.blobStore.Put(ctx, resourceblob.PutRequest{
		ResourceID:   resourceID,
		Name:         name,
		MimeType:     mimeType,
		Source:       reader,
		MetadataJSON: req.GetMetadataJson(),
	})
	if err != nil {
		return nil, mapResourceBlobError(err)
	}

	if _, err := s.exportStore.Create(ctx, exporteddomain.Record{
		ResourceID: resourceID,

		BlobStoreKind: blob.StoreKind,
		BlobID:        blob.StoreBlobID,

		Name:        blob.Name,
		MimeType:    blob.MimeType,
		SizeBytes:   blob.SizeBytes,
		ContentHash: blob.ContentHash,

		SourceServiceWorkspaceID: workspace.ID,
		SourceWorkspacePath:      exported.Source.Path,
	}); err != nil {
		_ = s.blobStore.Delete(ctx, resourceID)
		if errors.Is(err, exporteddomain.ErrAlreadyExists) {
			return nil, status.Error(codes.AlreadyExists, "exported resource already exists")
		}
		return nil, status.Errorf(codes.Internal, "create exported resource record: %v", err)
	}

	return &sandboxv1.ExportWorkspacePathResponse{
		Source: &sandboxv1.WorkspacePathRef{
			Workspace: &workspacev1.WorkspaceHostRef{
				ServiceId:          s.serviceID,
				ServiceWorkspaceId: workspace.ID,
				SandboxProfileId:   workspace.SandboxProfileID,
			},
			Path:        exported.Source.Path,
			Kind:        exported.Source.Kind,
			DisplayName: exported.Source.Name,
		},
		Resource: &resourcev1.ResourceRef{
			Id:                 resourceID,
			AuthorityServiceId: s.serviceID,
			Name:               blob.Name,
			MimeType:           blob.MimeType,
			SizeBytes:          blob.SizeBytes,
			ContentHash:        blob.ContentHash,
			MetadataJson:       append([]byte(nil), req.GetMetadataJson()...),
		},
	}, nil
}

type importResourceStreamReader struct {
	stream sandboxv1.WorkspaceTransferService_ImportResourceToWorkspaceServer
	buf    []byte
}

func (r *importResourceStreamReader) Read(p []byte) (int, error) {
	for len(r.buf) == 0 {
		msg, err := r.stream.Recv()
		if err != nil {
			if err == io.EOF {
				return 0, io.EOF
			}
			return 0, err
		}
		if msg.GetHeader() != nil {
			return 0, status.Error(codes.InvalidArgument, "import header must only be sent once")
		}
		data := msg.GetData()
		if len(data) == 0 {
			continue
		}
		r.buf = data
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

func importConflictPolicy(policy sandboxv1.ImportConflictPolicy) workspacestore.ImportConflictPolicy {
	if policy == sandboxv1.ImportConflictPolicy_IMPORT_CONFLICT_POLICY_OVERWRITE {
		return workspacestore.ImportConflictOverwrite
	}
	return workspacestore.ImportConflictFailIfExists
}

func (s *WorkspaceTransferService) exportableFile(ctx context.Context, serviceWorkspaceID string, inputPath string) (workspacedomain.Workspace, *workspacestore.ExportedPath, error) {
	if serviceWorkspaceID == "" {
		return workspacedomain.Workspace{}, nil, status.Error(codes.InvalidArgument, "service_workspace_id is required")
	}
	workspace, err := s.workspaceStore.Get(ctx, serviceWorkspaceID)
	if errors.Is(err, workspacedomain.ErrNotFound) {
		return workspacedomain.Workspace{}, nil, status.Error(codes.NotFound, "hosted workspace not found")
	}
	if err != nil {
		return workspacedomain.Workspace{}, nil, status.Errorf(codes.Internal, "get hosted workspace: %v", err)
	}
	if workspace.StoreWorkspaceID == "" {
		workspace.StoreWorkspaceID = workspace.ID
	}
	exported, err := s.backing.ExportPath(ctx, workspacestore.ExportPathRequest{
		WorkspaceID: workspace.StoreWorkspaceID,
		Path:        inputPath,
	})
	if err != nil {
		return workspacedomain.Workspace{}, nil, mapWorkspaceStoreError(err)
	}
	return workspace, exported, nil
}

func newExportedResourceID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "res_" + hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("res_%d", time.Now().UnixNano())
}
