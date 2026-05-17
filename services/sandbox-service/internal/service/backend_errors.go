package service

import (
	"errors"

	backenddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/backend"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func mapBackendError(err error) error {
	switch {
	case errors.Is(err, backenddomain.ErrInvalidRequest):
		return status.Error(codes.InvalidArgument, "invalid exec request")
	case errors.Is(err, backenddomain.ErrUnsupportedAttachment):
		return status.Error(codes.FailedPrecondition, "unsupported workspace attachment")
	case errors.Is(err, backenddomain.ErrAttachmentNotReady):
		return status.Error(codes.FailedPrecondition, "workspace attachment is not ready")
	case errors.Is(err, backenddomain.ErrInvalidCWD):
		return status.Error(codes.InvalidArgument, "invalid workspace cwd")
	case errors.Is(err, backenddomain.ErrExecTimeout):
		return status.Error(codes.DeadlineExceeded, "workspace exec timed out")
	case errors.Is(err, backenddomain.ErrExecStart):
		return status.Error(codes.FailedPrecondition, "workspace exec command failed to start")
	default:
		return status.Errorf(codes.Internal, "workspace exec failed: %v", err)
	}
}
