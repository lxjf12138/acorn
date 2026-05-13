package resource

import (
	"context"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
)

type Catalog interface {
	ListResources(ctx context.Context, workspaceID string) ([]*resourcev1.ResourceRef, error)
}
