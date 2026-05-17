package service

import (
	"context"

	leasedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacelease"
)

type fakeLeaseManager struct {
	acquireErr error
	releaseErr error

	lastAcquire  leasedomain.AcquireRequest
	released     bool
	releaseCount int
}

func (f *fakeLeaseManager) TryAcquire(_ context.Context, req leasedomain.AcquireRequest) (*leasedomain.Lease, error) {
	f.lastAcquire = req
	if f.acquireErr != nil {
		return nil, f.acquireErr
	}
	return &leasedomain.Lease{ID: "lease-1", WorkspaceID: req.WorkspaceID, Mode: req.Mode}, nil
}

func (f *fakeLeaseManager) Release(context.Context, *leasedomain.Lease) error {
	f.released = true
	f.releaseCount++
	return f.releaseErr
}
