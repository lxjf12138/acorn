package node

import (
	"context"

	nodev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/node/v1"
)

type Catalog interface {
	ListNodes(ctx context.Context) ([]*nodev1.Node, error)
	GetNode(ctx context.Context, nodeID string) (*nodev1.Node, error)
}
