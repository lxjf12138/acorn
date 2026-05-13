package workspace

import (
	"context"

	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
)

type Reader interface {
	GetWorkspace(ctx context.Context, workspaceID string) (*workspacev1.Workspace, error)
	ListFiles(ctx context.Context, workspaceID string, path string) ([]*workspacev1.FileInfo, error)
	ReadFile(ctx context.Context, workspaceID string, path string) ([]byte, error)
}
