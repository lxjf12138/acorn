package backend

import (
	"context"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/attachment"
)

type SandboxBackend interface {
	ID() string
	Kind() string

	Acquire(ctx context.Context, req AcquireRequest) (*SandboxLease, error)
	Release(ctx context.Context, lease *SandboxLease) error

	Exec(ctx context.Context, lease *SandboxLease, req ExecRequest) (*ExecResult, error)
}

type AcquireRequest struct {
	WorkspaceID string
	Attachment  *attachment.WorkspaceAttachment
	ProfileID   string
	Metadata    map[string]string
}
