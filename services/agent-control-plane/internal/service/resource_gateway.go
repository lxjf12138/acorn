package service

import (
	"context"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	resourcedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResourceAuthorityClient interface {
	OpenResource(ctx context.Context, resourceID string) (resourcev1.ResourceContentService_OpenResourceClient, error)
}

type ResourceGatewayService struct {
	store       resourcedomain.Store
	authorities map[string]ResourceAuthorityClient
}

func NewResourceGatewayService(store resourcedomain.Store, authorities map[string]ResourceAuthorityClient) *ResourceGatewayService {
	copied := make(map[string]ResourceAuthorityClient, len(authorities))
	for serviceID, client := range authorities {
		copied[serviceID] = client
	}
	return &ResourceGatewayService{
		store:       store,
		authorities: copied,
	}
}

func (s *ResourceGatewayService) OpenResource(ctx context.Context, resourceID string, userID string) (*resourcev1.ResourceRecord, resourcev1.ResourceContentService_OpenResourceClient, error) {
	if resourceID == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "resource_id is required")
	}
	record, err := s.store.Get(ctx, resourceID)
	if err != nil {
		return nil, nil, err
	}
	if record.GetStatus() != resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE {
		return nil, nil, status.Error(codes.FailedPrecondition, "resource is not available")
	}
	if record.GetOwnerUserId() != "" && userID != "" && record.GetOwnerUserId() != userID {
		return nil, nil, status.Error(codes.PermissionDenied, "resource owner mismatch")
	}
	ref := record.GetRef()
	if ref == nil || ref.GetAuthorityServiceId() == "" {
		return nil, nil, status.Error(codes.FailedPrecondition, "resource authority is missing")
	}
	authority, ok := s.authorities[ref.GetAuthorityServiceId()]
	if !ok {
		return nil, nil, status.Errorf(codes.FailedPrecondition, "unsupported resource authority: %s", ref.GetAuthorityServiceId())
	}
	stream, err := authority.OpenResource(ctx, resourceID)
	if err != nil {
		return nil, nil, err
	}
	return record, stream, nil
}
