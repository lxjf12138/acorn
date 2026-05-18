package service

import (
	"context"
	"encoding/json"
	"testing"

	eventdomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/event"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/infra/eventstore"
	"go.opentelemetry.io/otel/trace"
)

func TestEventServiceAppend(t *testing.T) {
	store := eventstore.NewMemoryStore()
	service := NewEventService("agent-control-plane", store)
	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	}))

	record, err := service.Append(ctx, AppendEventInput{
		Type:       eventdomain.TypeResourceUploaded,
		UserID:     "user-1",
		SessionID:  "sess-1",
		ResourceID: "res-1",
		Actor:      eventdomain.EventActor{Type: "user", ID: "user-1"},
		Subject:    eventdomain.EventSubject{Type: "resource", ID: "res-1"},
		Payload:    map[string]any{"size_bytes": int64(12)},
	})
	if err != nil {
		t.Fatalf("Append returned error: %v", err)
	}
	if record.ID == "" || record.ServiceID != "agent-control-plane" || record.TraceID == "" || record.SpanID == "" {
		t.Fatalf("unexpected record metadata: %+v", record)
	}
	var payload map[string]any
	if err := json.Unmarshal(record.PayloadJSON, &payload); err != nil {
		t.Fatalf("payload was not JSON: %v", err)
	}
	if payload["size_bytes"].(float64) != 12 {
		t.Fatalf("unexpected payload: %v", payload)
	}
}

func TestEventServiceList(t *testing.T) {
	store := eventstore.NewMemoryStore()
	service := NewEventService("agent-control-plane", store)
	if _, err := service.Append(context.Background(), AppendEventInput{Type: eventdomain.TypeWorkspaceCreated, SessionID: "s1"}); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}
	result, err := service.List(context.Background(), eventdomain.ListFilter{SessionID: "s1"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result.Events) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
