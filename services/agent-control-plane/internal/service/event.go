package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lxjf12138/acorn/packages/core/telemetry"
	eventdomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/event"
)

type EventAppender interface {
	Append(ctx context.Context, input AppendEventInput) (*eventdomain.EventRecord, error)
}

type EventService struct {
	serviceID string
	store     eventdomain.Store
}

type AppendEventInput struct {
	Type     string
	Severity eventdomain.Severity

	TenantID    string
	UserID      string
	SessionID   string
	WorkspaceID string
	ResourceID  string

	Actor   eventdomain.EventActor
	Subject eventdomain.EventSubject

	Payload any
}

func NewEventService(serviceID string, store eventdomain.Store) *EventService {
	return &EventService{serviceID: serviceID, store: store}
}

func (s *EventService) Append(ctx context.Context, input AppendEventInput) (*eventdomain.EventRecord, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	severity := input.Severity
	if severity == "" {
		severity = eventdomain.SeverityInfo
	}
	payloadJSON, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, err
	}
	if string(payloadJSON) == "null" {
		payloadJSON = nil
	}
	traceID, spanID := telemetry.TraceContext(ctx)
	record := &eventdomain.EventRecord{
		ID:          newEventID(),
		Type:        input.Type,
		Severity:    severity,
		OccurredAt:  time.Now().UTC(),
		ServiceID:   s.serviceID,
		TenantID:    input.TenantID,
		UserID:      input.UserID,
		SessionID:   input.SessionID,
		WorkspaceID: input.WorkspaceID,
		ResourceID:  input.ResourceID,
		TraceID:     traceID,
		SpanID:      spanID,
		Actor:       input.Actor,
		Subject:     input.Subject,
		PayloadJSON: payloadJSON,
	}
	if err := s.store.Append(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *EventService) List(ctx context.Context, filter eventdomain.ListFilter) (*eventdomain.ListResult, error) {
	if s == nil || s.store == nil {
		return &eventdomain.ListResult{}, nil
	}
	return s.store.List(ctx, filter)
}

func newEventID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "evt_" + hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}

func bestEffortAppendEvent(ctx context.Context, appender EventAppender, input AppendEventInput) {
	if appender == nil {
		return
	}
	_, _ = appender.Append(ctx, input)
}
