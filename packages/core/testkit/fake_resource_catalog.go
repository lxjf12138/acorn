package testkit

import (
	"context"
	"sync"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	resourcecore "github.com/lxjf12138/acorn/packages/core/resource"
	"google.golang.org/protobuf/proto"
)

type FakeResourceCatalog struct {
	mu        sync.RWMutex
	resources map[string][]*resourcev1.ResourceRef
}

func NewFakeResourceCatalog() *FakeResourceCatalog {
	return &FakeResourceCatalog{resources: make(map[string][]*resourcev1.ResourceRef)}
}

func (f *FakeResourceCatalog) AddResource(workspaceID string, ref *resourcev1.ResourceRef) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resources[workspaceID] = append(f.resources[workspaceID], proto.Clone(ref).(*resourcev1.ResourceRef))
}

func (f *FakeResourceCatalog) ListResources(_ context.Context, workspaceID string) ([]*resourcev1.ResourceRef, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	resources := f.resources[workspaceID]
	out := make([]*resourcev1.ResourceRef, 0, len(resources))
	for _, ref := range resources {
		out = append(out, proto.Clone(ref).(*resourcev1.ResourceRef))
	}
	return out, nil
}

var _ resourcecore.Catalog = (*FakeResourceCatalog)(nil)
