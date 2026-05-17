package workspacelease

import "context"

type Manager interface {
	TryAcquire(ctx context.Context, req AcquireRequest) (*Lease, error)
	Release(ctx context.Context, lease *Lease) error
}

type AcquireRequest struct {
	WorkspaceID string
	Mode        Mode

	Holder string
	Reason string

	Metadata map[string]string
}
