package service

import (
	"context"
	"errors"

	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	workspacestore "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WorkspaceViewService struct {
	sandboxv1.UnimplementedWorkspaceViewServiceServer

	serviceID string
	store     workspacedomain.Store
	backing   workspacestore.Store
}

func NewWorkspaceViewService(serviceID string, store workspacedomain.Store, backing workspacestore.Store) *WorkspaceViewService {
	return &WorkspaceViewService{
		serviceID: serviceID,
		store:     store,
		backing:   backing,
	}
}

func (s *WorkspaceViewService) ListWorkspaceDir(ctx context.Context, req *sandboxv1.ListWorkspaceDirRequest) (*sandboxv1.ListWorkspaceDirResponse, error) {
	workspace, err := s.workspace(ctx, req.GetServiceWorkspaceId())
	if err != nil {
		return nil, err
	}
	listing, err := s.backing.ListDir(ctx, workspacestore.ListDirRequest{
		WorkspaceID: workspace.StoreWorkspaceID,
		Path:        req.GetPath(),
		PageSize:    req.GetPageSize(),
		PageToken:   req.GetPageToken(),
	})
	if err != nil {
		return nil, mapWorkspaceStoreError(err)
	}
	out := make([]*sandboxv1.WorkspaceDirEntry, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		out = append(out, &sandboxv1.WorkspaceDirEntry{
			Name:       entry.Name,
			Ref:        s.pathRef(workspace, entry),
			Kind:       entry.Kind,
			SizeBytes:  entry.SizeBytes,
			ModifiedAt: timestampFromTime(entry.ModifiedAt),
		})
	}
	return &sandboxv1.ListWorkspaceDirResponse{
		Directory:     s.pathRef(workspace, listing.Directory),
		Entries:       out,
		NextPageToken: listing.NextToken,
	}, nil
}

func (s *WorkspaceViewService) PreviewWorkspaceFile(ctx context.Context, req *sandboxv1.PreviewWorkspaceFileRequest) (*sandboxv1.PreviewWorkspaceFileResponse, error) {
	workspace, err := s.workspace(ctx, req.GetServiceWorkspaceId())
	if err != nil {
		return nil, err
	}
	preview, err := s.backing.PreviewFile(ctx, workspacestore.PreviewFileRequest{
		WorkspaceID: workspace.StoreWorkspaceID,
		Path:        req.GetPath(),
		MaxBytes:    req.GetMaxBytes(),
	})
	if err != nil {
		return nil, mapWorkspaceStoreError(err)
	}
	return &sandboxv1.PreviewWorkspaceFileResponse{
		File:         s.pathRef(workspace, preview.File),
		MimeType:     preview.MimeType,
		PreviewBytes: append([]byte(nil), preview.Bytes...),
		Truncated:    preview.Truncated,
		SizeBytes:    preview.File.SizeBytes,
		ModifiedAt:   timestampFromTime(preview.File.ModifiedAt),
	}, nil
}

func (s *WorkspaceViewService) workspace(ctx context.Context, serviceWorkspaceID string) (workspacedomain.Workspace, error) {
	if serviceWorkspaceID == "" {
		return workspacedomain.Workspace{}, status.Error(codes.InvalidArgument, "service_workspace_id is required")
	}
	workspace, err := s.store.Get(ctx, serviceWorkspaceID)
	if errors.Is(err, workspacedomain.ErrNotFound) {
		return workspacedomain.Workspace{}, status.Error(codes.NotFound, "hosted workspace not found")
	}
	if err != nil {
		return workspacedomain.Workspace{}, status.Errorf(codes.Internal, "get hosted workspace: %v", err)
	}
	if workspace.StoreWorkspaceID == "" {
		workspace.StoreWorkspaceID = workspace.ID
	}
	return workspace, nil
}

func (s *WorkspaceViewService) pathRef(workspace workspacedomain.Workspace, pathInfo workspacestore.PathInfo) *sandboxv1.WorkspacePathRef {
	return &sandboxv1.WorkspacePathRef{
		Workspace: &workspacev1.WorkspaceHostRef{
			ServiceId:          s.serviceID,
			ServiceWorkspaceId: workspace.ID,
			SandboxProfileId:   workspace.SandboxProfileID,
		},
		Path:        pathInfo.Path,
		Kind:        pathInfo.Kind,
		DisplayName: pathInfo.Name,
	}
}
