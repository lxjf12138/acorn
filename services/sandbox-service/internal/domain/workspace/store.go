package workspace

import (
	"context"
	"errors"
	"sync"
)

var ErrNotFound = errors.New("workspace not found")

type Store interface {
	Create(ctx context.Context, workspace Workspace) (Workspace, error)
	Get(ctx context.Context, id string) (Workspace, error)
}

type MemoryStore struct {
	mu         sync.RWMutex
	workspaces map[string]Workspace
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{workspaces: make(map[string]Workspace)}
}

func (s *MemoryStore) Create(_ context.Context, workspace Workspace) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if workspace.ID == "" {
		workspace.ID = NewID()
	}
	s.workspaces[workspace.ID] = clone(workspace)
	return clone(workspace), nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspace, ok := s.workspaces[id]
	if !ok {
		return Workspace{}, ErrNotFound
	}
	return clone(workspace), nil
}

func clone(workspace Workspace) Workspace {
	if workspace.MetadataJSON != nil {
		workspace.MetadataJSON = append([]byte(nil), workspace.MetadataJSON...)
	}
	return workspace
}
