package service

import (
	"context"
	"errors"

	commonv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/common/v1"
	"github.com/lxjf12138/acorn/packages/core/telemetry"
	leasedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacelease"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func acquireWorkspaceLease(ctx context.Context, manager leasedomain.Manager, workspaceID string, mode leasedomain.Mode, reason string, scope *commonv1.Scope) (*leasedomain.Lease, error) {
	ctx, span := telemetry.Start(ctx, "sandbox-service/service", telemetry.SpanWorkspaceLeaseAcquire)
	defer span.End()
	span.SetAttributes(
		attribute.String(telemetry.AttrOperation, "workspace.lease.acquire"),
		attribute.String(telemetry.AttrLeaseMode, string(mode)),
		attribute.String(telemetry.AttrLeaseReason, reason),
	)
	if manager == nil {
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusOK))
		return nil, nil
	}
	lease, err := manager.TryAcquire(ctx, leasedomain.AcquireRequest{
		WorkspaceID: workspaceID,
		Mode:        mode,
		Holder:      leaseHolder(scope, reason),
		Reason:      reason,
	})
	if err != nil {
		mapped := mapWorkspaceLeaseError(err)
		telemetry.RecordError(span, mapped)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		if errors.Is(err, leasedomain.ErrWorkspaceBusy) {
			recordWorkspaceLeaseBusy(ctx, mode, reason)
		}
		return nil, mapped
	}
	span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusOK))
	return lease, nil
}

func releaseWorkspaceLease(ctx context.Context, manager leasedomain.Manager, lease *leasedomain.Lease) {
	if manager == nil || lease == nil {
		return
	}
	_ = manager.Release(ctx, lease)
}

func leaseHolder(scope *commonv1.Scope, fallback string) string {
	switch {
	case scope.GetToolCallId() != "":
		return "tool_call:" + scope.GetToolCallId()
	case scope.GetRunId() != "":
		return "run:" + scope.GetRunId()
	case scope.GetUserId() != "":
		return "user:" + scope.GetUserId()
	default:
		return fallback
	}
}

func mapWorkspaceLeaseError(err error) error {
	switch {
	case errors.Is(err, leasedomain.ErrWorkspaceIDRequired):
		return status.Error(codes.InvalidArgument, "workspace id required")
	case errors.Is(err, leasedomain.ErrInvalidMode):
		return status.Error(codes.InvalidArgument, "invalid workspace lease mode")
	case errors.Is(err, leasedomain.ErrWorkspaceBusy):
		return status.Error(codes.FailedPrecondition, "workspace is busy")
	case errors.Is(err, leasedomain.ErrLeaseNotFound):
		return status.Error(codes.NotFound, "workspace lease not found")
	default:
		return status.Errorf(codes.Internal, "workspace lease error: %v", err)
	}
}
