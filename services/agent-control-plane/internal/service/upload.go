package service

import (
	"context"
	"io"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	"github.com/lxjf12138/acorn/packages/core/events"
	resourceblob "github.com/lxjf12138/acorn/packages/core/resourceblob"
	"github.com/lxjf12138/acorn/packages/core/telemetry"
	"github.com/lxjf12138/acorn/packages/servicekit/eventing"
	"github.com/lxjf12138/acorn/packages/servicekit/httpx"
	resourcedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/resource"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultMaxUploadBytes = int64(100 * 1024 * 1024)

type UploadService struct {
	serviceID       string
	blobStore       resourceblob.Store
	resourceService resourceRegistrar
	maxUploadBytes  int64
	events          events.Emitter
}

type resourceRegistrar interface {
	RegisterRecord(ctx context.Context, req *resourcev1.RegisterResourceRequest) (*resourcev1.ResourceRecord, error)
}

type UploadResourceInput struct {
	UserID    string
	SessionID string

	Name     string
	MimeType string
	Source   io.Reader

	MetadataJSON []byte
}

func NewUploadService(serviceID string, blobStore resourceblob.Store, resourceService resourceRegistrar, maxUploadBytes int64) *UploadService {
	return NewUploadServiceWithEvents(serviceID, blobStore, resourceService, maxUploadBytes, eventing.NoopEmitter{})
}

func NewUploadServiceWithEvents(serviceID string, blobStore resourceblob.Store, resourceService resourceRegistrar, maxUploadBytes int64, emitter events.Emitter) *UploadService {
	if maxUploadBytes <= 0 {
		maxUploadBytes = defaultMaxUploadBytes
	}
	if emitter == nil {
		emitter = eventing.NoopEmitter{}
	}
	return &UploadService{
		serviceID:       serviceID,
		blobStore:       blobStore,
		resourceService: resourceService,
		maxUploadBytes:  maxUploadBytes,
		events:          emitter,
	}
}

func (s *UploadService) UploadResource(ctx context.Context, input UploadResourceInput) (*resourcev1.ResourceRecord, error) {
	ctx, span := telemetry.Start(ctx, "agent-control-plane/service", telemetry.SpanResourceUpload)
	defer span.End()
	span.SetAttributes(
		attribute.String(telemetry.AttrOperation, "resource.upload"),
		attribute.String(telemetry.AttrResourceMimeType, input.MimeType),
	)
	if input.UserID == "" {
		err := status.Error(codes.InvalidArgument, "user_id is required")
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusInvalid))
		recordResourceTransfer(ctx, "upload", telemetry.StatusInvalid, "", 0)
		return nil, err
	}
	if input.Source == nil {
		err := status.Error(codes.InvalidArgument, "upload source is required")
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusInvalid))
		recordResourceTransfer(ctx, "upload", telemetry.StatusInvalid, "", 0)
		return nil, err
	}
	resourceID := resourcedomain.NewRecordID()
	name := httpx.SafeFilename(input.Name, resourceID)
	blob, err := s.blobStore.Put(ctx, resourceblob.PutRequest{
		ResourceID:   resourceID,
		Name:         name,
		MimeType:     input.MimeType,
		Source:       &uploadLimitReader{reader: input.Source, remaining: s.maxUploadBytes},
		MetadataJSON: input.MetadataJSON,
	})
	if err != nil {
		if _, ok := status.FromError(err); ok {
			telemetry.RecordError(span, err)
			span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
			recordResourceTransfer(ctx, "upload", statusValue(err), "", 0)
			return nil, err
		}
		mapped := mapResourceBlobError(err)
		telemetry.RecordError(span, mapped)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		recordResourceTransfer(ctx, "upload", statusValue(mapped), "", 0)
		return nil, mapped
	}
	record, err := s.resourceService.RegisterRecord(ctx, &resourcev1.RegisterResourceRequest{
		Ref: &resourcev1.ResourceRef{
			Id:                 blob.ResourceID,
			AuthorityServiceId: s.serviceID,
			Name:               blob.Name,
			MimeType:           blob.MimeType,
			SizeBytes:          blob.SizeBytes,
			ContentHash:        blob.ContentHash,
			MetadataJson:       append([]byte(nil), input.MetadataJSON...),
		},
		OwnerUserId: input.UserID,
		SessionId:   input.SessionID,
		Source: &resourcev1.ResourceSource{
			Type:            "user_upload",
			SourceServiceId: s.serviceID,
		},
		Visibility:   resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE,
		MetadataJson: append([]byte(nil), input.MetadataJSON...),
	})
	if err != nil {
		_ = s.blobStore.Delete(ctx, resourceID)
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		recordResourceTransfer(ctx, "upload", statusValue(err), "", 0)
		return nil, err
	}
	span.SetAttributes(
		attribute.String(telemetry.AttrResourceAuthorityServiceID, record.GetRef().GetAuthorityServiceId()),
		attribute.Int64(telemetry.AttrResourceSizeBytes, record.GetRef().GetSizeBytes()),
		attribute.String(telemetry.AttrStatus, telemetry.StatusOK),
	)
	recordResourceTransfer(ctx, "upload", telemetry.StatusOK, record.GetRef().GetAuthorityServiceId(), record.GetRef().GetSizeBytes())
	s.events.Emit(ctx, events.Event{
		Name:     events.ResourceUploaded,
		Severity: events.SeverityInfo,
		Attributes: map[string]any{
			events.AttrResourceAuthorityServiceID: record.GetRef().GetAuthorityServiceId(),
			events.AttrResourceMimeType:           record.GetRef().GetMimeType(),
			events.AttrResourceSizeBytes:          record.GetRef().GetSizeBytes(),
		},
	})
	return record, nil
}

type uploadLimitReader struct {
	reader    io.Reader
	remaining int64
}

func (r *uploadLimitReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		var probe [1]byte
		n, err := r.reader.Read(probe[:])
		if n > 0 {
			return 0, status.Error(codes.ResourceExhausted, "upload too large")
		}
		return 0, err
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	r.remaining -= int64(n)
	if err == io.EOF {
		return n, io.EOF
	}
	if err != nil {
		return n, err
	}
	if r.remaining == 0 {
		var probe [1]byte
		extra, extraErr := r.reader.Read(probe[:])
		if extra > 0 {
			return n, status.Error(codes.ResourceExhausted, "upload too large")
		}
		if extraErr != nil && extraErr != io.EOF {
			return n, extraErr
		}
	}
	return n, nil
}
