package execution

import "context"

const (
	DefaultListLimit = 50
	MaxListLimit     = 200
)

type ListFilter struct {
	TenantID    string
	UserID      string
	SessionID   string
	WorkspaceID string
	Status      Status

	Limit     int
	PageToken string
}

type ListResult struct {
	Records       []*ExecutionRecord
	NextPageToken string
}

type Store interface {
	Create(ctx context.Context, record *ExecutionRecord) error
	Update(ctx context.Context, record *ExecutionRecord) error
	Get(ctx context.Context, id string) (*ExecutionRecord, error)
	List(ctx context.Context, filter ListFilter) (*ListResult, error)
}
