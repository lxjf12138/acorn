package exportedresource

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrAlreadyExists = errors.New("exported resource already exists")
	ErrNotFound      = errors.New("exported resource not found")
)

type Store interface {
	Create(ctx context.Context, record Record) (Record, error)
	Get(ctx context.Context, resourceID string) (Record, error)
}

type MemoryStore struct {
	mu      sync.RWMutex
	records map[string]Record
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{records: make(map[string]Record)}
}

func (s *MemoryStore) Create(_ context.Context, record Record) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[record.ResourceID]; ok {
		return Record{}, ErrAlreadyExists
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	s.records[record.ResourceID] = clone(record)
	return clone(record), nil
}

func (s *MemoryStore) Get(_ context.Context, resourceID string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[resourceID]
	if !ok {
		return Record{}, ErrNotFound
	}
	return clone(record), nil
}

func clone(record Record) Record {
	return record
}
