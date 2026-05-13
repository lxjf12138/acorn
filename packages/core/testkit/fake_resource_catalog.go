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
	resources []*resourcev1.ResourceRef
}

func NewFakeResourceCatalog() *FakeResourceCatalog {
	return &FakeResourceCatalog{}
}

func (f *FakeResourceCatalog) AddResource(ref *resourcev1.ResourceRef) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resources = append(f.resources, proto.Clone(ref).(*resourcev1.ResourceRef))
}

func (f *FakeResourceCatalog) ListResources(_ context.Context, filter resourcecore.ListFilter) ([]*resourcev1.ResourceRef, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]*resourcev1.ResourceRef, 0, len(f.resources))
	for _, ref := range f.resources {
		if filter.OwnerType != "" && ref.GetOwnerType() != filter.OwnerType {
			continue
		}
		if filter.OwnerID != "" && ref.GetOwnerId() != filter.OwnerID {
			continue
		}
		if filter.Type != "" && ref.GetType() != filter.Type {
			continue
		}
		if filter.Scope != nil {
			if serviceID := filter.Scope.GetServiceId(); serviceID != "" && ref.GetServiceId() != serviceID {
				continue
			}
			if providerID := filter.Scope.GetProviderId(); providerID != "" && ref.GetProviderId() != providerID {
				continue
			}
		}
		out = append(out, proto.Clone(ref).(*resourcev1.ResourceRef))
	}
	return out, nil
}

var _ resourcecore.Catalog = (*FakeResourceCatalog)(nil)
