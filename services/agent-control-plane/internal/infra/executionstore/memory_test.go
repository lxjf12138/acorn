package executionstore

import (
	"context"
	"errors"
	"testing"

	executiondomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/execution"
)

func TestMemoryStoreCreateGetUpdate(t *testing.T) {
	store := NewMemoryStore()
	record := &executiondomain.ExecutionRecord{
		ID:        "exec_1",
		SessionID: "sess-1",
		UserID:    "user-1",
		Status:    executiondomain.StatusRunning,
	}
	if err := store.Create(context.Background(), record); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if record.StartedAt.IsZero() || record.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps: %+v", record)
	}
	got, err := store.Get(context.Background(), "exec_1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	got.Status = executiondomain.StatusSucceeded
	got.ExitCode = 0
	if err := store.Update(context.Background(), got); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	updated, err := store.Get(context.Background(), "exec_1")
	if err != nil {
		t.Fatalf("Get updated returned error: %v", err)
	}
	if updated.Status != executiondomain.StatusSucceeded {
		t.Fatalf("unexpected status: %s", updated.Status)
	}
}

func TestMemoryStoreValidationAndMissing(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Create(context.Background(), nil); !errors.Is(err, executiondomain.ErrRecordRequired) {
		t.Fatalf("expected ErrRecordRequired, got %v", err)
	}
	if err := store.Create(context.Background(), &executiondomain.ExecutionRecord{}); !errors.Is(err, executiondomain.ErrIDRequired) {
		t.Fatalf("expected ErrIDRequired, got %v", err)
	}
	record := &executiondomain.ExecutionRecord{ID: "exec_1", SessionID: "sess-1", Status: executiondomain.StatusRunning}
	if err := store.Create(context.Background(), record); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := store.Create(context.Background(), record); !errors.Is(err, executiondomain.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
	if err := store.Update(context.Background(), &executiondomain.ExecutionRecord{ID: "missing", SessionID: "sess-1", Status: executiondomain.StatusRunning}); !errors.Is(err, executiondomain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if _, err := store.Get(context.Background(), "missing"); !errors.Is(err, executiondomain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStoreListFiltersAndPagination(t *testing.T) {
	store := NewMemoryStore()
	records := []*executiondomain.ExecutionRecord{
		{ID: "exec_1", SessionID: "sess-1", UserID: "user-1", WorkspaceID: "workspace-1", Status: executiondomain.StatusSucceeded},
		{ID: "exec_2", SessionID: "sess-1", UserID: "user-2", WorkspaceID: "workspace-1", Status: executiondomain.StatusFailed},
		{ID: "exec_3", SessionID: "sess-2", UserID: "user-1", WorkspaceID: "workspace-2", Status: executiondomain.StatusSucceeded},
	}
	for _, record := range records {
		if err := store.Create(context.Background(), record); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
	}
	result, err := store.List(context.Background(), executiondomain.ListFilter{SessionID: "sess-1", Limit: 1})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result.Records) != 1 || result.Records[0].ID != "exec_2" || result.NextPageToken != "1" {
		t.Fatalf("unexpected first page: %+v", result)
	}
	next, err := store.List(context.Background(), executiondomain.ListFilter{SessionID: "sess-1", Limit: 1, PageToken: result.NextPageToken})
	if err != nil {
		t.Fatalf("List next returned error: %v", err)
	}
	if len(next.Records) != 1 || next.Records[0].ID != "exec_1" || next.NextPageToken != "" {
		t.Fatalf("unexpected next page: %+v", next)
	}
	userResult, err := store.List(context.Background(), executiondomain.ListFilter{UserID: "user-1"})
	if err != nil {
		t.Fatalf("List user returned error: %v", err)
	}
	if len(userResult.Records) != 2 {
		t.Fatalf("expected 2 user records, got %d", len(userResult.Records))
	}
	statusResult, err := store.List(context.Background(), executiondomain.ListFilter{Status: executiondomain.StatusFailed})
	if err != nil {
		t.Fatalf("List status returned error: %v", err)
	}
	if len(statusResult.Records) != 1 || statusResult.Records[0].ID != "exec_2" {
		t.Fatalf("unexpected status records: %+v", statusResult)
	}
}

func TestMemoryStoreListRejectsInvalidPaging(t *testing.T) {
	store := NewMemoryStore()
	if _, err := store.List(context.Background(), executiondomain.ListFilter{Limit: -1}); !errors.Is(err, executiondomain.ErrInvalidLimit) {
		t.Fatalf("expected ErrInvalidLimit, got %v", err)
	}
	if _, err := store.List(context.Background(), executiondomain.ListFilter{PageToken: "wat"}); !errors.Is(err, executiondomain.ErrInvalidPageToken) {
		t.Fatalf("expected ErrInvalidPageToken, got %v", err)
	}
}
