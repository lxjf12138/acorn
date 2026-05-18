package service

import (
	"context"
	"errors"
	"time"

	"github.com/lxjf12138/acorn/packages/core/telemetry"
	executiondomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/execution"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ExecutionService struct {
	store executiondomain.Store
}

type StartExecutionInput struct {
	TenantID  string
	UserID    string
	SessionID string

	WorkspaceID        string
	ServiceWorkspaceID string

	SandboxServiceID string
	SandboxProfileID string
	SandboxBackendID string

	CommandName string
	ArgCount    int
	CWDSet      bool
}

type CompleteExecutionInput struct {
	ExitCode int32

	StdoutSizeBytes int64
	StderrSizeBytes int64
	StdoutTruncated bool
	StderrTruncated bool
}

type FailExecutionInput struct {
	Err error
}

func NewExecutionService(store executiondomain.Store) *ExecutionService {
	return &ExecutionService{store: store}
}

func (s *ExecutionService) Start(ctx context.Context, input StartExecutionInput) (*executiondomain.ExecutionRecord, error) {
	if s == nil || s.store == nil {
		return nil, status.Error(codes.FailedPrecondition, "execution store is not configured")
	}
	traceID, spanID := telemetry.TraceContext(ctx)
	now := time.Now().UTC()
	record := &executiondomain.ExecutionRecord{
		ID:                 executiondomain.NewRecordID(),
		TenantID:           input.TenantID,
		UserID:             input.UserID,
		SessionID:          input.SessionID,
		WorkspaceID:        input.WorkspaceID,
		ServiceWorkspaceID: input.ServiceWorkspaceID,
		SandboxServiceID:   input.SandboxServiceID,
		SandboxProfileID:   input.SandboxProfileID,
		SandboxBackendID:   input.SandboxBackendID,
		Status:             executiondomain.StatusRunning,
		CommandName:        input.CommandName,
		ArgCount:           input.ArgCount,
		CWDSet:             input.CWDSet,
		TraceID:            traceID,
		SpanID:             spanID,
		StartedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.store.Create(ctx, record); err != nil {
		return nil, mapExecutionError(err)
	}
	return record, nil
}

func (s *ExecutionService) Complete(ctx context.Context, id string, input CompleteExecutionInput) (*executiondomain.ExecutionRecord, error) {
	record, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if input.ExitCode == 0 {
		record.Status = executiondomain.StatusSucceeded
	} else {
		record.Status = executiondomain.StatusFailed
	}
	record.ExitCode = input.ExitCode
	record.StdoutSizeBytes = input.StdoutSizeBytes
	record.StderrSizeBytes = input.StderrSizeBytes
	record.StdoutTruncated = input.StdoutTruncated
	record.StderrTruncated = input.StderrTruncated
	record.CompletedAt = time.Now().UTC()
	record.UpdatedAt = record.CompletedAt
	if err := s.store.Update(ctx, record); err != nil {
		return nil, mapExecutionError(err)
	}
	return record, nil
}

func (s *ExecutionService) Fail(ctx context.Context, id string, input FailExecutionInput) (*executiondomain.ExecutionRecord, error) {
	record, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	record.Status = executiondomain.StatusFailed
	if status.Code(input.Err) == codes.DeadlineExceeded {
		record.Status = executiondomain.StatusTimeout
	}
	record.ErrorCode = status.Code(input.Err).String()
	record.ErrorMessage = safeExecutionErrorMessage(input.Err)
	record.CompletedAt = time.Now().UTC()
	record.UpdatedAt = record.CompletedAt
	if err := s.store.Update(ctx, record); err != nil {
		return nil, mapExecutionError(err)
	}
	return record, nil
}

func (s *ExecutionService) Get(ctx context.Context, id string) (*executiondomain.ExecutionRecord, error) {
	if s == nil || s.store == nil {
		return nil, status.Error(codes.FailedPrecondition, "execution store is not configured")
	}
	record, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, mapExecutionError(err)
	}
	return record, nil
}

func (s *ExecutionService) List(ctx context.Context, filter executiondomain.ListFilter) (*executiondomain.ListResult, error) {
	if s == nil || s.store == nil {
		return nil, status.Error(codes.FailedPrecondition, "execution store is not configured")
	}
	result, err := s.store.List(ctx, filter)
	if err != nil {
		return nil, mapExecutionError(err)
	}
	return result, nil
}

func mapExecutionError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, executiondomain.ErrIDRequired),
		errors.Is(err, executiondomain.ErrSessionRequired),
		errors.Is(err, executiondomain.ErrStatusRequired),
		errors.Is(err, executiondomain.ErrRecordRequired),
		errors.Is(err, executiondomain.ErrInvalidLimit),
		errors.Is(err, executiondomain.ErrInvalidPageToken):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, executiondomain.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, executiondomain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func safeExecutionErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if st, ok := status.FromError(err); ok {
		return st.Message()
	}
	return err.Error()
}
