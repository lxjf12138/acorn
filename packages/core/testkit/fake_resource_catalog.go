package testkit

import (
	"context"
	"errors"
	"sync"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	resourcecore "github.com/lxjf12138/acorn/packages/core/resource"
	"google.golang.org/protobuf/proto"
)

type FakeResourceCatalog struct {
	mu        sync.RWMutex
	resources map[string]*resourcev1.ResourceRecord
}

func NewFakeResourceCatalog() *FakeResourceCatalog {
	return &FakeResourceCatalog{
		resources: make(map[string]*resourcev1.ResourceRecord),
	}
}

func (f *FakeResourceCatalog) Register(_ context.Context, record *resourcev1.ResourceRecord) (*resourcev1.ResourceRecord, error) {
	if record == nil || record.GetRef() == nil || record.GetRef().GetId() == "" || record.GetRef().GetAuthorityServiceId() == "" {
		return nil, resourcecore.ErrInvalidResource
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	resourceID := record.GetRef().GetId()
	if _, ok := f.resources[resourceID]; ok {
		return nil, resourcecore.ErrAlreadyExists
	}
	cloned := proto.Clone(record).(*resourcev1.ResourceRecord)
	f.resources[resourceID] = cloned
	return proto.Clone(cloned).(*resourcev1.ResourceRecord), nil
}

func (f *FakeResourceCatalog) Get(_ context.Context, resourceID string) (*resourcev1.ResourceRecord, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	record, ok := f.resources[resourceID]
	if !ok {
		return nil, resourcecore.ErrResourceNotFound
	}
	return proto.Clone(record).(*resourcev1.ResourceRecord), nil
}

func (f *FakeResourceCatalog) List(_ context.Context, filter resourcecore.ListFilter) ([]*resourcev1.ResourceRecord, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]*resourcev1.ResourceRecord, 0, len(f.resources))
	for _, record := range f.resources {
		if filter.OwnerUserID != "" && record.GetOwnerUserId() != filter.OwnerUserID {
			continue
		}
		if filter.SessionID != "" && record.GetSessionId() != filter.SessionID {
			continue
		}
		if filter.Status != resourcev1.ResourceStatus_RESOURCE_STATUS_UNSPECIFIED && record.GetStatus() != filter.Status {
			continue
		}
		if filter.Visibility != resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_UNSPECIFIED && record.GetVisibility() != filter.Visibility {
			continue
		}
		out = append(out, proto.Clone(record).(*resourcev1.ResourceRecord))
	}
	return out, nil
}

func (f *FakeResourceCatalog) AddResource(record *resourcev1.ResourceRecord) {
	if _, err := f.Register(context.Background(), record); err != nil && !errors.Is(err, resourcecore.ErrAlreadyExists) {
		panic(err)
	}
}

var _ resourcecore.Catalog = (*FakeResourceCatalog)(nil)
