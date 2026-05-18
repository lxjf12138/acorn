package event

import "context"

const (
	DefaultListLimit = 100
	MaxListLimit     = 500
)

type Store interface {
	Append(ctx context.Context, record *EventRecord) error
	List(ctx context.Context, filter ListFilter) (*ListResult, error)
}

type ListFilter struct {
	TenantID    string
	UserID      string
	SessionID   string
	WorkspaceID string
	ResourceID  string

	Limit     int
	PageToken string
}

type ListResult struct {
	Events        []*EventRecord
	NextPageToken string
}
