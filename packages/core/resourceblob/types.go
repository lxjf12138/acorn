package resourceblob

import (
	"context"
	"io"
	"time"
)

type Store interface {
	Kind() string

	Put(ctx context.Context, req PutRequest) (*StoredBlob, error)
	Open(ctx context.Context, resourceID string) (io.ReadCloser, *BlobInfo, error)
	Stat(ctx context.Context, resourceID string) (*BlobInfo, error)
	Delete(ctx context.Context, resourceID string) error
}

type PutRequest struct {
	ResourceID string
	Name       string
	MimeType   string
	Source     io.Reader

	MetadataJSON []byte
}

type StoredBlob struct {
	ResourceID  string
	StoreKind   string
	StoreBlobID string

	Name        string
	MimeType    string
	SizeBytes   int64
	ContentHash string

	CreatedAt    time.Time
	MetadataJSON []byte
}

type BlobInfo struct {
	ResourceID  string
	StoreKind   string
	StoreBlobID string

	Name        string
	MimeType    string
	SizeBytes   int64
	ContentHash string

	CreatedAt    time.Time
	MetadataJSON []byte
}
