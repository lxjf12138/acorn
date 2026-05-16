package testkit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	resourcecore "github.com/lxjf12138/acorn/packages/core/resource"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	normalized, err := normalizeResourceRecord(record)
	if err != nil {
		return nil, err
	}
	resourceID := normalized.GetRef().GetId()

	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.resources[resourceID]; ok {
		return nil, resourcecore.ErrAlreadyExists
	}
	f.resources[resourceID] = normalized
	return proto.Clone(normalized).(*resourcev1.ResourceRecord), nil
}

func normalizeResourceRecord(record *resourcev1.ResourceRecord) (*resourcev1.ResourceRecord, error) {
	if record == nil || record.GetRef() == nil || record.GetRef().GetAuthorityServiceId() == "" || record.GetRef().GetName() == "" {
		return nil, resourcecore.ErrInvalidResource
	}
	normalized := proto.Clone(record).(*resourcev1.ResourceRecord)
	if normalized.GetRef().GetId() == "" {
		normalized.GetRef().Id = newResourceID()
	}
	if normalized.GetStatus() == resourcev1.ResourceStatus_RESOURCE_STATUS_UNSPECIFIED {
		normalized.Status = resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE
	}
	if normalized.GetVisibility() == resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_UNSPECIFIED {
		normalized.Visibility = resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_INTERNAL
	}
	now := time.Now().UTC()
	if normalized.GetCreatedAt() == nil {
		normalized.CreatedAt = timestamppb.New(now)
	}
	if normalized.GetUpdatedAt() == nil {
		normalized.UpdatedAt = timestamppb.New(now)
	}
	return normalized, nil
}

func newResourceID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "res_" + hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("res_%d", time.Now().UnixNano())
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
