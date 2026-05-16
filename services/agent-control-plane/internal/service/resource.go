package service

import (
	"context"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	resourcedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResourceService struct {
	resourcev1.UnimplementedResourceServiceServer
	store resourcedomain.Store
}

func NewResourceService(store resourcedomain.Store) *ResourceService {
	return &ResourceService{store: store}
}

func (s *ResourceService) RegisterResource(ctx context.Context, req *resourcev1.RegisterResourceRequest) (*resourcev1.RegisterResourceResponse, error) {
	record, err := s.register(ctx, req)
	if err != nil {
		return nil, err
	}
	return &resourcev1.RegisterResourceResponse{Resource: record}, nil
}

func (s *ResourceService) GetResource(ctx context.Context, req *resourcev1.GetResourceRequest) (*resourcev1.GetResourceResponse, error) {
	record, err := s.get(ctx, req.GetResourceId())
	if err != nil {
		return nil, err
	}
	return &resourcev1.GetResourceResponse{Resource: record}, nil
}

func (s *ResourceService) ListResources(ctx context.Context, req *resourcev1.ListResourcesRequest) (*resourcev1.ListResourcesResponse, error) {
	records, err := s.list(ctx, resourcedomain.Filter{
		OwnerUserID: req.GetOwnerUserId(),
		SessionID:   req.GetSessionId(),
		Status:      req.GetStatus(),
		Visibility:  req.GetVisibility(),
	})
	if err != nil {
		return nil, err
	}
	return &resourcev1.ListResourcesResponse{Resources: records}, nil
}

func (s *ResourceService) RegisterRecord(ctx context.Context, req *resourcev1.RegisterResourceRequest) (*resourcev1.ResourceRecord, error) {
	return s.register(ctx, req)
}

func (s *ResourceService) GetRecord(ctx context.Context, resourceID string) (*resourcev1.ResourceRecord, error) {
	return s.get(ctx, resourceID)
}

func (s *ResourceService) ListRecords(ctx context.Context, filter resourcedomain.Filter) ([]*resourcev1.ResourceRecord, error) {
	return s.list(ctx, filter)
}

func (s *ResourceService) register(ctx context.Context, req *resourcev1.RegisterResourceRequest) (*resourcev1.ResourceRecord, error) {
	ownerUserID := req.GetOwnerUserId()
	if ownerUserID == "" {
		ownerUserID = req.GetScope().GetUserId()
	}
	if ownerUserID == "" {
		return nil, status.Error(codes.InvalidArgument, "owner_user_id or scope.user_id is required")
	}
	sessionID := req.GetSessionId()
	if sessionID == "" {
		sessionID = req.GetScope().GetSessionId()
	}
	record := &resourcev1.ResourceRecord{
		Ref:          cloneResourceRef(req.GetRef()),
		OwnerUserId:  ownerUserID,
		SessionId:    sessionID,
		Source:       cloneResourceSource(req.GetSource()),
		Visibility:   req.GetVisibility(),
		MetadataJson: append([]byte(nil), req.GetMetadataJson()...),
	}
	return s.store.Register(ctx, record)
}

func (s *ResourceService) get(ctx context.Context, resourceID string) (*resourcev1.ResourceRecord, error) {
	return s.store.Get(ctx, resourceID)
}

func (s *ResourceService) list(ctx context.Context, filter resourcedomain.Filter) ([]*resourcev1.ResourceRecord, error) {
	return s.store.List(ctx, filter)
}

func cloneResourceRef(ref *resourcev1.ResourceRef) *resourcev1.ResourceRef {
	if ref == nil {
		return nil
	}
	return &resourcev1.ResourceRef{
		Id:                 ref.GetId(),
		AuthorityServiceId: ref.GetAuthorityServiceId(),
		Name:               ref.GetName(),
		MimeType:           ref.GetMimeType(),
		SizeBytes:          ref.GetSizeBytes(),
		ContentHash:        ref.GetContentHash(),
		MetadataJson:       append([]byte(nil), ref.GetMetadataJson()...),
	}
}

func cloneResourceSource(source *resourcev1.ResourceSource) *resourcev1.ResourceSource {
	if source == nil {
		return nil
	}
	return &resourcev1.ResourceSource{
		Type:               source.GetType(),
		SourceServiceId:    source.GetSourceServiceId(),
		WorkspaceRecordId:  source.GetWorkspaceRecordId(),
		ServiceWorkspaceId: source.GetServiceWorkspaceId(),
		SourcePath:         source.GetSourcePath(),
		RunId:              source.GetRunId(),
		ToolCallId:         source.GetToolCallId(),
		MetadataJson:       append([]byte(nil), source.GetMetadataJson()...),
	}
}
