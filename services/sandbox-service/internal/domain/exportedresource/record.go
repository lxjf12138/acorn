package exportedresource

import "time"

type Record struct {
	ResourceID         string
	ServiceWorkspaceID string
	WorkspacePath      string
	Name               string
	MimeType           string
	SizeBytes          int64
	CreatedAt          time.Time
}
