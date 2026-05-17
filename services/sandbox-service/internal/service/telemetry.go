package service

import (
	"errors"

	"github.com/lxjf12138/acorn/packages/core/telemetry"
	leasedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacelease"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func statusValue(err error) string {
	if errors.Is(err, leasedomain.ErrWorkspaceBusy) ||
		(status.Code(err) == codes.FailedPrecondition && status.Convert(err).Message() == "workspace is busy") {
		return telemetry.StatusBusy
	}
	switch status.Code(err) {
	case codes.OK:
		return telemetry.StatusOK
	case codes.InvalidArgument:
		return telemetry.StatusInvalid
	case codes.PermissionDenied:
		return telemetry.StatusDenied
	default:
		return telemetry.StatusError
	}
}
