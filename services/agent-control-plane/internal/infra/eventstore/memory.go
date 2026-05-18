package eventstore

import (
	"context"
	"strconv"
	"sync"
	"time"

	eventdomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/event"
)

type MemoryStore struct {
	mu     sync.RWMutex
	events []*eventdomain.EventRecord
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) Append(_ context.Context, record *eventdomain.EventRecord) error {
	if record == nil || record.ID == "" {
		return eventdomain.ErrEventIDRequired
	}
	if record.Type == "" {
		return eventdomain.ErrEventTypeRequired
	}
	normalized := cloneEvent(record)
	if normalized.OccurredAt.IsZero() {
		normalized.OccurredAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, normalized)
	return nil
}

func (s *MemoryStore) List(_ context.Context, filter eventdomain.ListFilter) (*eventdomain.ListResult, error) {
	limit := filter.Limit
	if limit == 0 {
		limit = eventdomain.DefaultListLimit
	}
	if limit < 0 || limit > eventdomain.MaxListLimit {
		return nil, eventdomain.ErrInvalidLimit
	}
	offset := 0
	if filter.PageToken != "" {
		parsed, err := strconv.Atoi(filter.PageToken)
		if err != nil || parsed < 0 {
			return nil, eventdomain.ErrInvalidLimit
		}
		offset = parsed
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	matched := make([]*eventdomain.EventRecord, 0, len(s.events))
	for i := len(s.events) - 1; i >= 0; i-- {
		record := s.events[i]
		if !matches(record, filter) {
			continue
		}
		matched = append(matched, record)
	}
	if offset >= len(matched) {
		return &eventdomain.ListResult{}, nil
	}
	end := offset + limit
	if end > len(matched) {
		end = len(matched)
	}
	out := make([]*eventdomain.EventRecord, 0, end-offset)
	for _, record := range matched[offset:end] {
		out = append(out, cloneEvent(record))
	}
	next := ""
	if end < len(matched) {
		next = strconv.Itoa(end)
	}
	return &eventdomain.ListResult{Events: out, NextPageToken: next}, nil
}

func matches(record *eventdomain.EventRecord, filter eventdomain.ListFilter) bool {
	if filter.TenantID != "" && record.TenantID != filter.TenantID {
		return false
	}
	if filter.UserID != "" && record.UserID != filter.UserID {
		return false
	}
	if filter.SessionID != "" && record.SessionID != filter.SessionID {
		return false
	}
	if filter.WorkspaceID != "" && record.WorkspaceID != filter.WorkspaceID {
		return false
	}
	if filter.ResourceID != "" && record.ResourceID != filter.ResourceID {
		return false
	}
	return true
}

func cloneEvent(record *eventdomain.EventRecord) *eventdomain.EventRecord {
	if record == nil {
		return nil
	}
	out := *record
	out.PayloadJSON = append([]byte(nil), record.PayloadJSON...)
	return &out
}
