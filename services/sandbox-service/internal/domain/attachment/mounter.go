package attachment

import "context"

type WorkspaceMounter interface {
	Kind() string
	Prepare(ctx context.Context, req PrepareRequest) (*WorkspaceAttachment, error)
	Release(ctx context.Context, attachment *WorkspaceAttachment) error
}

type PrepareRequest struct {
	WorkspaceID string
	Target      Target
	ReadOnly    bool
}
