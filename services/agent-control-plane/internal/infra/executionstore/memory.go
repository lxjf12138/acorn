package executionstore

import (
	"context"
	"strconv"
	"sync"
	"time"

	executiondomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/execution"
)

type MemoryStore struct {
	mu      sync.RWMutex
	records map[string]*executiondomain.ExecutionRecord
	order   []string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{records: make(map[string]*executiondomain.ExecutionRecord)}
}

func (s *MemoryStore) Create(_ context.Context, record *executiondomain.ExecutionRecord) error {
	if err := validateRecord(record); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[record.ID]; ok {
		return executiondomain.ErrAlreadyExists
	}
	now := time.Now().UTC()
	normalized := cloneRecord(record)
	if normalized.StartedAt.IsZero() {
		normalized.StartedAt = now
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = now
	}
	s.records[normalized.ID] = normalized
	s.order = append(s.order, normalized.ID)
	*record = *cloneRecord(normalized)
	return nil
}

func (s *MemoryStore) Update(_ context.Context, record *executiondomain.ExecutionRecord) error {
	if err := validateRecord(record); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[record.ID]; !ok {
		return executiondomain.ErrNotFound
	}
	normalized := cloneRecord(record)
	normalized.UpdatedAt = time.Now().UTC()
	s.records[normalized.ID] = normalized
	*record = *cloneRecord(normalized)
	return nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (*executiondomain.ExecutionRecord, error) {
	if id == "" {
		return nil, executiondomain.ErrIDRequired
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[id]
	if !ok {
		return nil, executiondomain.ErrNotFound
	}
	return cloneRecord(record), nil
}

func (s *MemoryStore) List(_ context.Context, filter executiondomain.ListFilter) (*executiondomain.ListResult, error) {
	limit, offset, err := normalizeList(filter)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	matches := make([]*executiondomain.ExecutionRecord, 0, len(s.records))
	for i := len(s.order) - 1; i >= 0; i-- {
		record := s.records[s.order[i]]
		if !matchesFilter(record, filter) {
			continue
		}
		matches = append(matches, cloneRecord(record))
	}
	if offset > len(matches) {
		offset = len(matches)
	}
	end := offset + limit
	if end > len(matches) {
		end = len(matches)
	}
	result := &executiondomain.ListResult{Records: matches[offset:end]}
	if end < len(matches) {
		result.NextPageToken = strconv.Itoa(end)
	}
	return result, nil
}

func validateRecord(record *executiondomain.ExecutionRecord) error {
	if record == nil {
		return executiondomain.ErrRecordRequired
	}
	if record.ID == "" {
		return executiondomain.ErrIDRequired
	}
	if record.SessionID == "" {
		return executiondomain.ErrSessionRequired
	}
	if record.Status == "" {
		return executiondomain.ErrStatusRequired
	}
	return nil
}

func normalizeList(filter executiondomain.ListFilter) (int, int, error) {
	limit := filter.Limit
	if limit == 0 {
		limit = executiondomain.DefaultListLimit
	}
	if limit < 0 || limit > executiondomain.MaxListLimit {
		return 0, 0, executiondomain.ErrInvalidLimit
	}
	if filter.PageToken == "" {
		return limit, 0, nil
	}
	offset, err := strconv.Atoi(filter.PageToken)
	if err != nil || offset < 0 {
		return 0, 0, executiondomain.ErrInvalidPageToken
	}
	return limit, offset, nil
}

func matchesFilter(record *executiondomain.ExecutionRecord, filter executiondomain.ListFilter) bool {
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
	if filter.Status != "" && record.Status != filter.Status {
		return false
	}
	return true
}

func cloneRecord(record *executiondomain.ExecutionRecord) *executiondomain.ExecutionRecord {
	if record == nil {
		return nil
	}
	clone := *record
	return &clone
}
