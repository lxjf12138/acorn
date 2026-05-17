package service

import (
	"errors"
	"time"

	workspacestore "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapWorkspaceStoreError(err error) error {
	switch {
	case errors.Is(err, workspacestore.ErrInvalidPath):
		return status.Error(codes.InvalidArgument, "invalid workspace path")
	case errors.Is(err, workspacestore.ErrPathNotFound):
		return status.Error(codes.NotFound, "workspace path not found")
	case errors.Is(err, workspacestore.ErrWorkspaceNotFound):
		return status.Error(codes.NotFound, "workspace backing store not found")
	case errors.Is(err, workspacestore.ErrPathNotDirectory):
		return status.Error(codes.FailedPrecondition, "workspace path is not a directory")
	case errors.Is(err, workspacestore.ErrPathIsDirectory):
		return status.Error(codes.FailedPrecondition, "workspace path is a directory")
	case errors.Is(err, workspacestore.ErrPathNotRegularFile):
		return status.Error(codes.FailedPrecondition, "workspace path is not a regular file")
	case errors.Is(err, workspacestore.ErrPathAlreadyExists):
		return status.Error(codes.AlreadyExists, "workspace path already exists")
	case errors.Is(err, workspacestore.ErrSymlinkNotAllowed):
		return status.Error(codes.PermissionDenied, "workspace symlink is not allowed")
	case errors.Is(err, workspacestore.ErrPathEscapesWorkspace):
		return status.Error(codes.PermissionDenied, "workspace path escapes workspace root")
	case errors.Is(err, workspacestore.ErrContentHashMismatch):
		return status.Error(codes.DataLoss, "workspace import content hash mismatch")
	case errors.Is(err, workspacestore.ErrImportTooLarge):
		return status.Error(codes.ResourceExhausted, "workspace import too large")
	case errors.Is(err, workspacestore.ErrParentNotFound):
		return status.Error(codes.NotFound, "workspace parent path not found")
	case errors.Is(err, workspacestore.ErrWorkspaceNotReady):
		return status.Error(codes.FailedPrecondition, "workspace backing store is not ready")
	default:
		return status.Errorf(codes.Internal, "workspace store operation failed: %v", err)
	}
}

func timestampFromTime(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}
