package testkit

import (
	"context"
	"testing"

	nodev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/node/v1"
)

func TestFakeNodeCatalogListsNodes(t *testing.T) {
	catalog := NewFakeNodeCatalog()
	catalog.AddNode(&nodev1.Node{Id: "node-1", Name: "Node One"})

	nodes, err := catalog.ListNodes(context.Background())
	if err != nil {
		t.Fatalf("ListNodes returned error: %v", err)
	}
	if len(nodes) != 1 || nodes[0].GetId() != "node-1" {
		t.Fatalf("unexpected nodes: %+v", nodes)
	}
}
