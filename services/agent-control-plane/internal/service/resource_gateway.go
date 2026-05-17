package service

import (
	"context"
	"io"

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

type ResourceStream struct {
	record *resourcev1.ResourceRecord
	stream resourcev1.ResourceContentService_OpenResourceClient
	first  *resourcev1.OpenResourceResponse
	buf    []byte
}

func (s *ResourceStream) Record() *resourcev1.ResourceRecord {
	return s.record
}

func (s *ResourceStream) FirstChunk() *resourcev1.OpenResourceResponse {
	return s.first
}

func (s *ResourceStream) Recv() (*resourcev1.OpenResourceResponse, error) {
	if s.first != nil {
		chunk := s.first
		s.first = nil
		return chunk, nil
	}
	return s.stream.Recv()
}

func (s *ResourceStream) Read(p []byte) (int, error) {
	if len(s.buf) > 0 {
		n := copy(p, s.buf)
		s.buf = s.buf[n:]
		return n, nil
	}
	for {
		chunk, err := s.Recv()
		if err != nil {
			return 0, err
		}
		if len(chunk.GetData()) == 0 {
			continue
		}
		s.buf = chunk.GetData()
		return s.Read(p)
	}
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

func (s *ResourceGatewayService) OpenResource(ctx context.Context, resourceID string, userID string) (*resourcev1.ResourceRecord, *ResourceStream, error) {
	resourceStream, err := s.OpenResourceForTransfer(ctx, resourceID, userID)
	if err != nil {
		return nil, nil, err
	}
	return resourceStream.record, resourceStream, nil
}

func (s *ResourceGatewayService) OpenResourceForTransfer(ctx context.Context, resourceID string, userID string) (*ResourceStream, error) {
	if resourceID == "" {
		return nil, status.Error(codes.InvalidArgument, "resource_id is required")
	}
	record, err := s.store.Get(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	if record.GetStatus() != resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE {
		return nil, status.Error(codes.FailedPrecondition, "resource is not available")
	}
	if record.GetOwnerUserId() != "" && userID != "" && record.GetOwnerUserId() != userID {
		return nil, status.Error(codes.PermissionDenied, "resource owner mismatch")
	}
	ref := record.GetRef()
	if ref == nil || ref.GetAuthorityServiceId() == "" {
		return nil, status.Error(codes.FailedPrecondition, "resource authority is missing")
	}
	authority, ok := s.authorities[ref.GetAuthorityServiceId()]
	if !ok {
		return nil, status.Errorf(codes.FailedPrecondition, "unsupported resource authority: %s", ref.GetAuthorityServiceId())
	}
	stream, err := authority.OpenResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}
	first, err := stream.Recv()
	if err != nil {
		if err == io.EOF {
			return nil, status.Error(codes.Internal, "resource authority returned no content")
		}
		return nil, err
	}
	if err := ValidateStreamResourceRef(ref, first.GetResource()); err != nil {
		return nil, err
	}
	return &ResourceStream{record: record, stream: stream, first: first}, nil
}

func ValidateStreamResourceRef(recordRef *resourcev1.ResourceRef, streamRef *resourcev1.ResourceRef) error {
	if streamRef == nil {
		return nil
	}
	if recordRef == nil {
		return status.Error(codes.Internal, "resource record has no ref")
	}
	if streamRef.GetId() != "" && streamRef.GetId() != recordRef.GetId() {
		return status.Error(codes.Internal, "resource authority returned mismatched resource id")
	}
	if streamRef.GetAuthorityServiceId() != "" && streamRef.GetAuthorityServiceId() != recordRef.GetAuthorityServiceId() {
		return status.Error(codes.Internal, "resource authority returned mismatched authority_service_id")
	}
	if streamRef.GetContentHash() != "" && recordRef.GetContentHash() != "" && streamRef.GetContentHash() != recordRef.GetContentHash() {
		return status.Error(codes.Internal, "resource authority returned mismatched content_hash")
	}
	if streamRef.GetSizeBytes() > 0 && recordRef.GetSizeBytes() > 0 && streamRef.GetSizeBytes() != recordRef.GetSizeBytes() {
		return status.Error(codes.Internal, "resource authority returned mismatched size_bytes")
	}
	return nil
}
