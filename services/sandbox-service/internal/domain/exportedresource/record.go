package exportedresource

import "time"

type Record struct {
	ResourceID string

	BlobStoreKind string
	BlobID        string

	Name        string
	MimeType    string
	SizeBytes   int64
	ContentHash string

	SourceServiceWorkspaceID string
	SourceWorkspacePath      string

	CreatedAt time.Time
}
