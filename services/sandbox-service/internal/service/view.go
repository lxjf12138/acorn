package service

import (
	"context"
	"errors"

	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	"github.com/lxjf12138/acorn/packages/core/telemetry"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	leasedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacelease"
	workspacestore "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacestore"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WorkspaceViewService struct {
	sandboxv1.UnimplementedWorkspaceViewServiceServer

	serviceID string
	store     workspacedomain.Store
	backing   workspacestore.Store
	leases    leasedomain.Manager
}

func NewWorkspaceViewService(serviceID string, store workspacedomain.Store, backing workspacestore.Store, leases leasedomain.Manager) *WorkspaceViewService {
	return &WorkspaceViewService{
		serviceID: serviceID,
		store:     store,
		backing:   backing,
		leases:    leases,
	}
}

func (s *WorkspaceViewService) ListWorkspaceDir(ctx context.Context, req *sandboxv1.ListWorkspaceDirRequest) (*sandboxv1.ListWorkspaceDirResponse, error) {
	ctx, span := telemetry.Start(ctx, "sandbox-service/service", telemetry.SpanWorkspaceViewList)
	defer span.End()
	span.SetAttributes(attribute.String(telemetry.AttrOperation, "workspace.view.list"))
	workspace, err := s.workspace(ctx, req.GetServiceWorkspaceId())
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		return nil, err
	}
	span.SetAttributes(attribute.String(telemetry.AttrWorkspaceProfileID, workspace.SandboxProfileID))
	lease, err := acquireWorkspaceLease(ctx, s.leases, workspace.ID, leasedomain.ModeRead, "list_workspace_dir", req.GetScope())
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		return nil, err
	}
	defer releaseWorkspaceLease(ctx, s.leases, lease)
	listing, err := s.backing.ListDir(ctx, workspacestore.ListDirRequest{
		WorkspaceID: workspace.StoreWorkspaceID,
		Path:        req.GetPath(),
		PageSize:    req.GetPageSize(),
		PageToken:   req.GetPageToken(),
	})
	if err != nil {
		mapped := mapWorkspaceStoreError(err)
		telemetry.RecordError(span, mapped)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(mapped)))
		return nil, mapped
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
	span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusOK))
	return &sandboxv1.ListWorkspaceDirResponse{
		Directory:     s.pathRef(workspace, listing.Directory),
		Entries:       out,
		NextPageToken: listing.NextToken,
	}, nil
}

func (s *WorkspaceViewService) PreviewWorkspaceFile(ctx context.Context, req *sandboxv1.PreviewWorkspaceFileRequest) (*sandboxv1.PreviewWorkspaceFileResponse, error) {
	ctx, span := telemetry.Start(ctx, "sandbox-service/service", telemetry.SpanWorkspaceViewPreview)
	defer span.End()
	span.SetAttributes(attribute.String(telemetry.AttrOperation, "workspace.view.preview"))
	workspace, err := s.workspace(ctx, req.GetServiceWorkspaceId())
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		return nil, err
	}
	span.SetAttributes(attribute.String(telemetry.AttrWorkspaceProfileID, workspace.SandboxProfileID))
	lease, err := acquireWorkspaceLease(ctx, s.leases, workspace.ID, leasedomain.ModeRead, "preview_workspace_file", req.GetScope())
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		return nil, err
	}
	defer releaseWorkspaceLease(ctx, s.leases, lease)
	preview, err := s.backing.PreviewFile(ctx, workspacestore.PreviewFileRequest{
		WorkspaceID: workspace.StoreWorkspaceID,
		Path:        req.GetPath(),
		MaxBytes:    req.GetMaxBytes(),
	})
	if err != nil {
		mapped := mapWorkspaceStoreError(err)
		telemetry.RecordError(span, mapped)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(mapped)))
		return nil, mapped
	}
	span.SetAttributes(
		attribute.String(telemetry.AttrResourceMimeType, preview.MimeType),
		attribute.Int64(telemetry.AttrResourceSizeBytes, preview.File.SizeBytes),
		attribute.Bool(telemetry.AttrTruncated, preview.Truncated),
		attribute.String(telemetry.AttrStatus, telemetry.StatusOK),
	)
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
