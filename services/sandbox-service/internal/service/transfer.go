package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	exporteddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/exportedresource"
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
	exportStore    exporteddomain.Store
}

func NewWorkspaceTransferService(serviceID string, workspaceStore workspacedomain.Store, backing workspacestore.Store, exportStore exporteddomain.Store) *WorkspaceTransferService {
	return &WorkspaceTransferService{
		serviceID:      serviceID,
		workspaceStore: workspaceStore,
		backing:        backing,
		exportStore:    exportStore,
	}
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

	if _, err := s.exportStore.Create(ctx, exporteddomain.Record{
		ResourceID:         resourceID,
		ServiceWorkspaceID: workspace.ID,
		WorkspacePath:      exported.Source.Path,
		Name:               name,
		MimeType:           mimeType,
		SizeBytes:          exported.SizeBytes,
	}); err != nil {
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
			Name:               name,
			MimeType:           mimeType,
			SizeBytes:          exported.SizeBytes,
			MetadataJson:       append([]byte(nil), req.GetMetadataJson()...),
		},
	}, nil
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
