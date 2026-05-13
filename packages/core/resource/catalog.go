package resource

import (
	"context"

	commonv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/common/v1"
	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
)

type ListFilter struct {
	Scope     *commonv1.Scope
	OwnerType string
	OwnerID   string
	Type      string
}

type Catalog interface {
	ListResources(ctx context.Context, filter ListFilter) ([]*resourcev1.ResourceRef, error)
}
