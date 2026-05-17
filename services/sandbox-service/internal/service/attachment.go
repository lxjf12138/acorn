package service

import (
	"context"
	"errors"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/attachment"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WorkspaceAttachmentService struct {
	workspaceStore workspacedomain.Store
	mounter        attachment.WorkspaceMounter
}

func NewWorkspaceAttachmentService(workspaceStore workspacedomain.Store, mounter attachment.WorkspaceMounter) *WorkspaceAttachmentService {
	return &WorkspaceAttachmentService{
		workspaceStore: workspaceStore,
		mounter:        mounter,
	}
}

func (s *WorkspaceAttachmentService) PrepareLocalProcessAttachment(ctx context.Context, serviceWorkspaceID string, readOnly bool) (*attachment.WorkspaceAttachment, error) {
	if serviceWorkspaceID == "" {
		return nil, status.Error(codes.InvalidArgument, "service_workspace_id is required")
	}
	if s.mounter == nil {
		return nil, status.Error(codes.FailedPrecondition, "workspace mounter is not configured")
	}
	workspace, err := s.workspaceStore.Get(ctx, serviceWorkspaceID)
	if errors.Is(err, workspacedomain.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "hosted workspace not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get hosted workspace: %v", err)
	}
	storeWorkspaceID := workspace.StoreWorkspaceID
	if storeWorkspaceID == "" {
		storeWorkspaceID = workspace.ID
	}
	att, err := s.mounter.Prepare(ctx, attachment.PrepareRequest{
		WorkspaceID: storeWorkspaceID,
		Target: attachment.Target{
			Kind: attachment.TargetLocalProcess,
		},
		ReadOnly: readOnly,
	})
	if err != nil {
		return nil, mapAttachmentError(err)
	}
	return att, nil
}

func mapAttachmentError(err error) error {
	switch {
	case errors.Is(err, attachment.ErrWorkspaceNotFound):
		return status.Error(codes.NotFound, "workspace attachment source not found")
	case errors.Is(err, attachment.ErrUnsupportedTarget):
		return status.Error(codes.FailedPrecondition, "unsupported workspace attachment target")
	case errors.Is(err, attachment.ErrAttachmentNotReady):
		return status.Error(codes.FailedPrecondition, "workspace attachment is not ready")
	default:
		return status.Errorf(codes.Internal, "prepare workspace attachment: %v", err)
	}
}
