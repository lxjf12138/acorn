package workspace

import (
	"context"
	"errors"
	"sync"
)

var ErrNotFound = errors.New("workspace record not found")

type Store interface {
	Create(ctx context.Context, record Record) (Record, error)
	Get(ctx context.Context, id string) (Record, error)
	GetBySession(ctx context.Context, sessionID string) (Record, bool, error)
}

type MemoryStore struct {
	mu           sync.RWMutex
	records      map[string]Record
	sessionIndex map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		records:      make(map[string]Record),
		sessionIndex: make(map[string]string),
	}
}

func (s *MemoryStore) Create(_ context.Context, record Record) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existingID, ok := s.sessionIndex[record.SessionID]; ok {
		return clone(s.records[existingID]), nil
	}
	if record.ID == "" {
		record.ID = NewRecordID()
	}
	s.records[record.ID] = clone(record)
	s.sessionIndex[record.SessionID] = record.ID
	return clone(record), nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	return clone(record), nil
}

func (s *MemoryStore) GetBySession(_ context.Context, sessionID string) (Record, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.sessionIndex[sessionID]
	if !ok {
		return Record{}, false, nil
	}
	return clone(s.records[id]), true, nil
}

func clone(record Record) Record {
	if record.CurrentHost != nil {
		host := *record.CurrentHost
		record.CurrentHost = &host
	}
	if record.MetadataJSON != nil {
		record.MetadataJSON = append([]byte(nil), record.MetadataJSON...)
	}
	return record
}
