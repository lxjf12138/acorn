package server

import (
	"context"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	eventdomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/event"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/infra/eventstore"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/service"
)

func TestListSessionEvents(t *testing.T) {
	events := service.NewEventService("agent-control-plane", eventstore.NewMemoryStore())
	if _, err := events.Append(context.Background(), service.AppendEventInput{
		Type:      eventdomain.TypeWorkspaceCreated,
		UserID:    "user-1",
		SessionID: "sess-1",
		Subject:   eventdomain.EventSubject{Type: "workspace", ID: "ws-1"},
		Actor:     eventdomain.EventActor{Type: "user", ID: "user-1"},
	}); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}
	if _, err := events.Append(context.Background(), service.AppendEventInput{
		Type:      eventdomain.TypeWorkspaceCreated,
		UserID:    "user-2",
		SessionID: "sess-1",
	}); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx := newDownloadTestContext("unused", "user-1", recorder)
	ctx.request = httptest.NewRequest(nethttp.MethodGet, "/sessions/sess-1/events?user_id=user-1&limit=10", nil)
	ctx.vars.Set("session_id", "sess-1")
	ctx.query = ctx.request.URL.Query()

	if err := listSessionEvents(ctx, events); err != nil {
		t.Fatalf("listSessionEvents returned error: %v", err)
	}
	var got listEventsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("response was not JSON: %v", err)
	}
	if len(got.Events) != 1 || got.Events[0].UserID != "user-1" || got.Events[0].Type != eventdomain.TypeWorkspaceCreated {
		t.Fatalf("unexpected events response: %+v", got)
	}
}
