package eventstore

import (
	"context"
	"errors"
	"testing"
	"time"

	eventdomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/event"
)

func TestMemoryStoreAppendAndList(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now().UTC()
	events := []*eventdomain.EventRecord{
		{ID: "evt_1", Type: eventdomain.TypeWorkspaceCreated, SessionID: "s1", WorkspaceID: "w1", OccurredAt: now},
		{ID: "evt_2", Type: eventdomain.TypeResourceUploaded, SessionID: "s1", ResourceID: "r1", OccurredAt: now.Add(time.Second)},
		{ID: "evt_3", Type: eventdomain.TypeResourceUploaded, SessionID: "s2", ResourceID: "r2", OccurredAt: now.Add(2 * time.Second)},
	}
	for _, event := range events {
		if err := store.Append(context.Background(), event); err != nil {
			t.Fatalf("Append returned error: %v", err)
		}
	}

	result, err := store.List(context.Background(), eventdomain.ListFilter{SessionID: "s1", Limit: 1})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result.Events) != 1 || result.Events[0].ID != "evt_2" || result.NextPageToken == "" {
		t.Fatalf("unexpected first page: %+v", result)
	}
	next, err := store.List(context.Background(), eventdomain.ListFilter{SessionID: "s1", Limit: 1, PageToken: result.NextPageToken})
	if err != nil {
		t.Fatalf("List next returned error: %v", err)
	}
	if len(next.Events) != 1 || next.Events[0].ID != "evt_1" || next.NextPageToken != "" {
		t.Fatalf("unexpected second page: %+v", next)
	}
}

func TestMemoryStoreValidation(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Append(context.Background(), &eventdomain.EventRecord{Type: "x"}); !errors.Is(err, eventdomain.ErrEventIDRequired) {
		t.Fatalf("expected ErrEventIDRequired, got %v", err)
	}
	if err := store.Append(context.Background(), &eventdomain.EventRecord{ID: "evt_1"}); !errors.Is(err, eventdomain.ErrEventTypeRequired) {
		t.Fatalf("expected ErrEventTypeRequired, got %v", err)
	}
	if _, err := store.List(context.Background(), eventdomain.ListFilter{Limit: eventdomain.MaxListLimit + 1}); !errors.Is(err, eventdomain.ErrInvalidLimit) {
		t.Fatalf("expected ErrInvalidLimit, got %v", err)
	}
}
