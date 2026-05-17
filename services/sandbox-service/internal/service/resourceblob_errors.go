package service

import (
	"errors"

	resourceblob "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/resourceblob"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func mapResourceBlobError(err error) error {
	switch {
	case errors.Is(err, resourceblob.ErrInvalidResourceID):
		return status.Error(codes.InvalidArgument, "invalid resource id")
	case errors.Is(err, resourceblob.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, "resource blob already exists")
	case errors.Is(err, resourceblob.ErrNotFound):
		return status.Error(codes.NotFound, "resource blob not found")
	case errors.Is(err, resourceblob.ErrStoreNotReady):
		return status.Error(codes.FailedPrecondition, "resource blob store is not ready")
	default:
		return status.Errorf(codes.Internal, "resource blob operation failed: %v", err)
	}
}
