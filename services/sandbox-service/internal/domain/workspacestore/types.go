package workspacestore

import (
	"context"
	"io"
	"time"

	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
)

type Store interface {
	Kind() string

	CreateBackingWorkspace(ctx context.Context, req CreateBackingWorkspaceRequest) (*BackingWorkspace, error)
	DeleteBackingWorkspace(ctx context.Context, workspaceID string) error

	ListDir(ctx context.Context, req ListDirRequest) (*DirListing, error)
	PreviewFile(ctx context.Context, req PreviewFileRequest) (*FilePreview, error)
	StatPath(ctx context.Context, req StatPathRequest) (*PathInfo, error)
	ExportPath(ctx context.Context, req ExportPathRequest) (*ExportedPath, error)
}

type CreateBackingWorkspaceRequest struct {
	WorkspaceID      string
	SandboxProfileID string
	DisplayName      string
	MetadataJSON     []byte
}

type BackingWorkspace struct {
	ID               string
	StoreKind        string
	StoreWorkspaceID string
	SandboxProfileID string
	Status           workspacev1.WorkspaceStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
	MetadataJSON     []byte
}

type ListDirRequest struct {
	WorkspaceID string
	Path        string
	PageSize    int32
	PageToken   string
}

type DirListing struct {
	Directory PathInfo
	Entries   []PathInfo
	NextToken string
}

type PreviewFileRequest struct {
	WorkspaceID string
	Path        string
	MaxBytes    int64
}

type FilePreview struct {
	File      PathInfo
	MimeType  string
	Bytes     []byte
	Truncated bool
}

type StatPathRequest struct {
	WorkspaceID string
	Path        string
}

type ExportPathRequest struct {
	WorkspaceID string
	Path        string
}

type ExportedPath struct {
	Source    PathInfo
	MimeType  string
	SizeBytes int64
	Open      func(ctx context.Context) (io.ReadCloser, error)
}

type PathInfo struct {
	Path       string
	Name       string
	Kind       sandboxv1.WorkspacePathKind
	SizeBytes  int64
	ModifiedAt time.Time
}
