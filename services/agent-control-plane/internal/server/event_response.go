package server

import (
	"encoding/json"
	"time"

	eventdomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/event"
)

type listEventsResponse struct {
	Events        []eventResponse `json:"events"`
	NextPageToken string          `json:"next_page_token,omitempty"`
}

type eventResponse struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Severity   string `json:"severity"`
	OccurredAt string `json:"occurred_at"`

	ServiceID string `json:"service_id"`

	TenantID    string `json:"tenant_id,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	ResourceID  string `json:"resource_id,omitempty"`

	TraceID string `json:"trace_id,omitempty"`
	SpanID  string `json:"span_id,omitempty"`

	Subject eventdomain.EventSubject `json:"subject"`
	Actor   eventdomain.EventActor   `json:"actor"`

	Payload json.RawMessage `json:"payload,omitempty"`
}

func eventListResponse(result *eventdomain.ListResult) listEventsResponse {
	if result == nil {
		return listEventsResponse{}
	}
	out := listEventsResponse{
		Events:        make([]eventResponse, 0, len(result.Events)),
		NextPageToken: result.NextPageToken,
	}
	for _, record := range result.Events {
		out.Events = append(out.Events, eventResponse{
			ID:          record.ID,
			Type:        record.Type,
			Severity:    string(record.Severity),
			OccurredAt:  record.OccurredAt.UTC().Format(time.RFC3339Nano),
			ServiceID:   record.ServiceID,
			TenantID:    record.TenantID,
			UserID:      record.UserID,
			SessionID:   record.SessionID,
			WorkspaceID: record.WorkspaceID,
			ResourceID:  record.ResourceID,
			TraceID:     record.TraceID,
			SpanID:      record.SpanID,
			Subject:     record.Subject,
			Actor:       record.Actor,
			Payload:     append(json.RawMessage(nil), record.PayloadJSON...),
		})
	}
	return out
}
