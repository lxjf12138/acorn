package service

import (
	"github.com/lxjf12138/acorn/packages/core/telemetry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func statusValue(err error) string {
	switch status.Code(err) {
	case codes.OK:
		return telemetry.StatusOK
	case codes.InvalidArgument:
		return telemetry.StatusInvalid
	case codes.PermissionDenied:
		return telemetry.StatusDenied
	case codes.DeadlineExceeded:
		return telemetry.StatusTimeout
	default:
		return telemetry.StatusError
	}
}
