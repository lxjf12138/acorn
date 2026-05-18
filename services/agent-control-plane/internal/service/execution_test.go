package service

import (
	"context"
	"testing"

	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/execution"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/infra/executionstore"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestExecutionServiceStartCompleteAndFail(t *testing.T) {
	service := NewExecutionService(executionstore.NewMemoryStore())
	ctx, span := otel.Tracer("test").Start(context.Background(), "test")
	defer span.End()

	record, err := service.Start(ctx, StartExecutionInput{
		UserID:             "user-1",
		SessionID:          "sess-1",
		WorkspaceID:        "workspace-1",
		ServiceWorkspaceID: "ws-1",
		SandboxServiceID:   "sandbox-service",
		SandboxProfileID:   "local-process",
		CommandName:        "go",
		ArgCount:           1,
		CWDSet:             true,
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if record.ID == "" || record.Status != execution.StatusRunning || record.CommandName != "go" || record.ArgCount != 1 || !record.CWDSet {
		t.Fatalf("unexpected start record: %+v", record)
	}
	if record.StartedAt.IsZero() || record.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps: %+v", record)
	}

	completed, err := service.Complete(context.Background(), record.ID, CompleteExecutionInput{
		ExitCode:        2,
		StdoutSizeBytes: 3,
		StderrSizeBytes: 4,
		StdoutTruncated: true,
		StderrTruncated: true,
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if completed.Status != execution.StatusFailed || completed.ExitCode != 2 || !completed.StdoutTruncated || !completed.StderrTruncated {
		t.Fatalf("unexpected completed record: %+v", completed)
	}
	if completed.CompletedAt.IsZero() {
		t.Fatalf("expected completed_at: %+v", completed)
	}

	timeout, err := service.Start(context.Background(), StartExecutionInput{SessionID: "sess-1", CommandName: "go"})
	if err != nil {
		t.Fatalf("Start timeout record returned error: %v", err)
	}
	failed, err := service.Fail(context.Background(), timeout.ID, FailExecutionInput{Err: status.Error(codes.DeadlineExceeded, "timeout")})
	if err != nil {
		t.Fatalf("Fail returned error: %v", err)
	}
	if failed.Status != execution.StatusTimeout || failed.ErrorCode != codes.DeadlineExceeded.String() || failed.ErrorMessage != "timeout" {
		t.Fatalf("unexpected failed record: %+v", failed)
	}
}

func TestExecutionServiceGetList(t *testing.T) {
	service := NewExecutionService(executionstore.NewMemoryStore())
	first, err := service.Start(context.Background(), StartExecutionInput{SessionID: "sess-1", UserID: "user-1", CommandName: "go"})
	if err != nil {
		t.Fatalf("Start first returned error: %v", err)
	}
	second, err := service.Start(context.Background(), StartExecutionInput{SessionID: "sess-1", UserID: "user-1", CommandName: "go"})
	if err != nil {
		t.Fatalf("Start second returned error: %v", err)
	}
	got, err := service.Get(context.Background(), first.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ID != first.ID {
		t.Fatalf("unexpected get record: %+v", got)
	}
	list, err := service.List(context.Background(), execution.ListFilter{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list.Records) != 2 || list.Records[0].ID != second.ID {
		t.Fatalf("unexpected list: %+v", list)
	}
}
