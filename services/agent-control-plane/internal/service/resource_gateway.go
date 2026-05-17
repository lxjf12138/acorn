package service

import (
	"context"
	"io"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	"github.com/lxjf12138/acorn/packages/core/telemetry"
	resourcedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/resource"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResourceAuthorityClient interface {
	OpenResource(ctx context.Context, resourceID string) (ResourceChunkStream, error)
}

type ResourceChunkStream interface {
	Recv() (*resourcev1.OpenResourceResponse, error)
}

type ResourceGatewayService struct {
	store       resourcedomain.Store
	authorities map[string]ResourceAuthorityClient
}

type ResourceStream struct {
	record *resourcev1.ResourceRecord
	stream ResourceChunkStream
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
	ctx, span := telemetry.Start(ctx, "agent-control-plane/service", telemetry.SpanResourceDownload)
	defer span.End()
	span.SetAttributes(attribute.String(telemetry.AttrOperation, "resource.download"))
	resourceStream, err := s.OpenResourceForTransfer(ctx, resourceID, userID)
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		return nil, nil, err
	}
	ref := resourceStream.record.GetRef()
	span.SetAttributes(
		attribute.String(telemetry.AttrResourceAuthorityServiceID, ref.GetAuthorityServiceId()),
		attribute.String(telemetry.AttrResourceMimeType, ref.GetMimeType()),
		attribute.Int64(telemetry.AttrResourceSizeBytes, ref.GetSizeBytes()),
		attribute.String(telemetry.AttrStatus, telemetry.StatusOK),
	)
	return resourceStream.record, resourceStream, nil
}

func (s *ResourceGatewayService) OpenResourceForTransfer(ctx context.Context, resourceID string, userID string) (*ResourceStream, error) {
	ctx, span := telemetry.Start(ctx, "agent-control-plane/service", telemetry.SpanResourceGatewayOpen)
	defer span.End()
	span.SetAttributes(attribute.String(telemetry.AttrOperation, "resource.gateway.open"))
	if resourceID == "" {
		err := status.Error(codes.InvalidArgument, "resource_id is required")
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusInvalid))
		return nil, err
	}
	record, err := s.store.Get(ctx, resourceID)
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		return nil, err
	}
	if record.GetStatus() != resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE {
		err := status.Error(codes.FailedPrecondition, "resource is not available")
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		return nil, err
	}
	if record.GetOwnerUserId() != "" && userID != "" && record.GetOwnerUserId() != userID {
		err := status.Error(codes.PermissionDenied, "resource owner mismatch")
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusDenied))
		return nil, err
	}
	ref := record.GetRef()
	if ref == nil || ref.GetAuthorityServiceId() == "" {
		err := status.Error(codes.FailedPrecondition, "resource authority is missing")
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		return nil, err
	}
	span.SetAttributes(
		attribute.String(telemetry.AttrResourceAuthorityServiceID, ref.GetAuthorityServiceId()),
		attribute.String(telemetry.AttrResourceMimeType, ref.GetMimeType()),
		attribute.Int64(telemetry.AttrResourceSizeBytes, ref.GetSizeBytes()),
	)
	authority, ok := s.authorities[ref.GetAuthorityServiceId()]
	if !ok {
		err := status.Errorf(codes.FailedPrecondition, "unsupported resource authority: %s", ref.GetAuthorityServiceId())
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		return nil, err
	}
	stream, err := authority.OpenResource(ctx, resourceID)
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		return nil, err
	}
	first, err := stream.Recv()
	if err != nil {
		if err == io.EOF {
			err = status.Error(codes.Internal, "resource authority returned no content")
		}
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		return nil, err
	}
	if err := ValidateStreamResourceRef(ref, first.GetResource()); err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		return nil, err
	}
	span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusOK))
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
