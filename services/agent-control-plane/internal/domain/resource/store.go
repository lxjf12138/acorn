package resource

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Filter struct {
	OwnerUserID string
	SessionID   string
	Status      resourcev1.ResourceStatus
	Visibility  resourcev1.ResourceVisibility
}

type Store interface {
	Register(ctx context.Context, record *resourcev1.ResourceRecord) (*resourcev1.ResourceRecord, error)
	Get(ctx context.Context, id string) (*resourcev1.ResourceRecord, error)
	List(ctx context.Context, filter Filter) ([]*resourcev1.ResourceRecord, error)
}

type MemoryStore struct {
	mu      sync.RWMutex
	records map[string]*resourcev1.ResourceRecord
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		records: make(map[string]*resourcev1.ResourceRecord),
	}
}

func (s *MemoryStore) Register(_ context.Context, record *resourcev1.ResourceRecord) (*resourcev1.ResourceRecord, error) {
	normalized, err := normalizeRecord(record)
	if err != nil {
		return nil, err
	}
	resourceID := normalized.GetRef().GetId()

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.records[resourceID]; ok {
		return nil, status.Error(codes.AlreadyExists, "resource already exists")
	}
	s.records[resourceID] = cloneRecord(normalized)
	return cloneRecord(normalized), nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (*resourcev1.ResourceRecord, error) {
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "resource_id is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[id]
	if !ok {
		return nil, status.Error(codes.NotFound, "resource not found")
	}
	return cloneRecord(record), nil
}

func (s *MemoryStore) List(_ context.Context, filter Filter) ([]*resourcev1.ResourceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*resourcev1.ResourceRecord, 0, len(s.records))
	for _, record := range s.records {
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
		out = append(out, cloneRecord(record))
	}
	return out, nil
}

func normalizeRecord(record *resourcev1.ResourceRecord) (*resourcev1.ResourceRecord, error) {
	if record == nil {
		return nil, status.Error(codes.InvalidArgument, "resource record is required")
	}
	normalized := cloneRecord(record)
	ref := normalized.GetRef()
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "resource ref is required")
	}
	if ref.GetId() == "" {
		ref.Id = NewRecordID()
	}
	if ref.GetAuthorityServiceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "authority_service_id is required")
	}
	if ref.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "resource name is required")
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

func NewRecordID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "res_" + hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("res_%d", time.Now().UnixNano())
}

func cloneRecord(record *resourcev1.ResourceRecord) *resourcev1.ResourceRecord {
	if record == nil {
		return nil
	}
	return proto.Clone(record).(*resourcev1.ResourceRecord)
}
