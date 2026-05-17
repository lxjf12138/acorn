package workspacelease

import (
	"context"
	"fmt"
	"sync"
	"time"

	leasedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacelease"
)

type MemoryManager struct {
	mu     sync.Mutex
	states map[string]*workspaceState
	nextID int64
}

type workspaceState struct {
	readers map[string]*leasedomain.Lease
	writer  *leasedomain.Lease
}

func NewMemoryManager() *MemoryManager {
	return &MemoryManager{states: map[string]*workspaceState{}}
}

func (m *MemoryManager) TryAcquire(_ context.Context, req leasedomain.AcquireRequest) (*leasedomain.Lease, error) {
	if req.WorkspaceID == "" {
		return nil, leasedomain.ErrWorkspaceIDRequired
	}
	if req.Mode != leasedomain.ModeRead && req.Mode != leasedomain.ModeWrite {
		return nil, leasedomain.ErrInvalidMode
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.states[req.WorkspaceID]
	if state == nil {
		state = &workspaceState{readers: map[string]*leasedomain.Lease{}}
		m.states[req.WorkspaceID] = state
	}
	switch req.Mode {
	case leasedomain.ModeRead:
		if state.writer != nil {
			return nil, leasedomain.ErrWorkspaceBusy
		}
	case leasedomain.ModeWrite:
		if state.writer != nil || len(state.readers) > 0 {
			return nil, leasedomain.ErrWorkspaceBusy
		}
	}
	m.nextID++
	lease := &leasedomain.Lease{
		ID:          fmt.Sprintf("lease_%d", m.nextID),
		WorkspaceID: req.WorkspaceID,
		Mode:        req.Mode,
		Holder:      req.Holder,
		Reason:      req.Reason,
		AcquiredAt:  time.Now().UTC(),
		Metadata:    cloneMap(req.Metadata),
	}
	if req.Mode == leasedomain.ModeWrite {
		state.writer = lease
	} else {
		state.readers[lease.ID] = lease
	}
	return lease.Clone(), nil
}

func (m *MemoryManager) Release(_ context.Context, lease *leasedomain.Lease) error {
	if lease == nil || lease.WorkspaceID == "" || lease.ID == "" {
		return leasedomain.ErrLeaseNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.states[lease.WorkspaceID]
	if state == nil {
		return leasedomain.ErrLeaseNotFound
	}
	switch lease.Mode {
	case leasedomain.ModeRead:
		if _, ok := state.readers[lease.ID]; !ok {
			return leasedomain.ErrLeaseNotFound
		}
		delete(state.readers, lease.ID)
	case leasedomain.ModeWrite:
		if state.writer == nil || state.writer.ID != lease.ID {
			return leasedomain.ErrLeaseNotFound
		}
		state.writer = nil
	default:
		return leasedomain.ErrInvalidMode
	}
	if state.writer == nil && len(state.readers) == 0 {
		delete(m.states, lease.WorkspaceID)
	}
	return nil
}

func cloneMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
