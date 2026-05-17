package service

import (
	"errors"
	"io"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	exporteddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/exportedresource"
	resourceblob "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/resourceblob"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultResourceChunkBytes = 64 * 1024

type ResourceContentService struct {
	resourcev1.UnimplementedResourceContentServiceServer

	serviceID   string
	exportStore exporteddomain.Store
	blobStore   resourceblob.Store
	chunkBytes  int
}

func NewResourceContentService(serviceID string, exportStore exporteddomain.Store, blobStore resourceblob.Store) *ResourceContentService {
	return &ResourceContentService{
		serviceID:   serviceID,
		exportStore: exportStore,
		blobStore:   blobStore,
		chunkBytes:  defaultResourceChunkBytes,
	}
}

func (s *ResourceContentService) OpenResource(req *resourcev1.OpenResourceRequest, stream resourcev1.ResourceContentService_OpenResourceServer) error {
	resourceID := req.GetResourceId()
	if resourceID == "" {
		return status.Error(codes.InvalidArgument, "resource_id is required")
	}
	record, err := s.exportStore.Get(stream.Context(), resourceID)
	if err != nil {
		if errors.Is(err, exporteddomain.ErrNotFound) {
			return status.Error(codes.NotFound, "exported resource not found")
		}
		return status.Errorf(codes.Internal, "get exported resource: %v", err)
	}

	blobID := record.BlobID
	if blobID == "" {
		blobID = resourceID
	}
	reader, info, err := s.blobStore.Open(stream.Context(), blobID)
	if err != nil {
		return mapResourceBlobError(err)
	}
	defer reader.Close()

	ref := &resourcev1.ResourceRef{
		Id:                 resourceID,
		AuthorityServiceId: s.serviceID,
		Name:               info.Name,
		MimeType:           info.MimeType,
		SizeBytes:          info.SizeBytes,
		ContentHash:        info.ContentHash,
		MetadataJson:       append([]byte(nil), info.MetadataJSON...),
	}
	if ref.Name == "" {
		ref.Name = record.Name
	}
	if ref.MimeType == "" {
		ref.MimeType = record.MimeType
	}
	if ref.SizeBytes == 0 {
		ref.SizeBytes = record.SizeBytes
	}
	if ref.ContentHash == "" {
		ref.ContentHash = record.ContentHash
	}

	buf := make([]byte, s.chunkBytes)
	first := true
	for {
		if err := stream.Context().Err(); err != nil {
			return err
		}
		n, readErr := reader.Read(buf)
		if n > 0 {
			chunk := &resourcev1.OpenResourceResponse{
				Data: append([]byte(nil), buf[:n]...),
			}
			if first {
				chunk.Resource = ref
				first = false
			}
			if err := stream.Send(chunk); err != nil {
				return err
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				if first {
					return stream.Send(&resourcev1.OpenResourceResponse{Resource: ref})
				}
				return nil
			}
			return status.Errorf(codes.Internal, "read resource blob: %v", readErr)
		}
	}
}
