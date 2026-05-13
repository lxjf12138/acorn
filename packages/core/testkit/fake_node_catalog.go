package testkit

import (
	"context"
	"sync"

	nodev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/node/v1"
	nodecore "github.com/lxjf12138/acorn/packages/core/node"
	"google.golang.org/protobuf/proto"
)

type FakeNodeCatalog struct {
	mu    sync.RWMutex
	nodes map[string]*nodev1.Node
}

func NewFakeNodeCatalog() *FakeNodeCatalog {
	return &FakeNodeCatalog{nodes: make(map[string]*nodev1.Node)}
}

func (f *FakeNodeCatalog) AddNode(node *nodev1.Node) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nodes[node.GetId()] = proto.Clone(node).(*nodev1.Node)
}

func (f *FakeNodeCatalog) ListNodes(context.Context) ([]*nodev1.Node, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]*nodev1.Node, 0, len(f.nodes))
	for _, node := range f.nodes {
		out = append(out, proto.Clone(node).(*nodev1.Node))
	}
	return out, nil
}

func (f *FakeNodeCatalog) GetNode(_ context.Context, nodeID string) (*nodev1.Node, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	node, ok := f.nodes[nodeID]
	if !ok {
		return nil, nodecore.ErrNodeNotFound
	}
	return proto.Clone(node).(*nodev1.Node), nil
}

var _ nodecore.Catalog = (*FakeNodeCatalog)(nil)
