package service

import (
	"context"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	resourcedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/resource"
)

type ResourceService struct {
	store resourcedomain.Store
}

func NewResourceService(store resourcedomain.Store) *ResourceService {
	return &ResourceService{store: store}
}

func (s *ResourceService) RegisterResource(ctx context.Context, req *resourcev1.RegisterResourceRequest) (*resourcev1.ResourceRecord, error) {
	record := &resourcev1.ResourceRecord{
		Ref:          cloneResourceRef(req.GetRef()),
		OwnerUserId:  req.GetOwnerUserId(),
		SessionId:    req.GetSessionId(),
		Source:       cloneResourceSource(req.GetSource()),
		Visibility:   req.GetVisibility(),
		MetadataJson: append([]byte(nil), req.GetMetadataJson()...),
	}
	return s.store.Register(ctx, record)
}

func (s *ResourceService) GetResource(ctx context.Context, resourceID string) (*resourcev1.ResourceRecord, error) {
	return s.store.Get(ctx, resourceID)
}

func (s *ResourceService) ListResources(ctx context.Context, filter resourcedomain.Filter) ([]*resourcev1.ResourceRecord, error) {
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
