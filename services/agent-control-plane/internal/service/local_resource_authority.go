package service

import (
	"context"
	"io"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	resourceblob "github.com/lxjf12138/acorn/packages/core/resourceblob"
)

const localAuthorityChunkBytes = 64 * 1024

type LocalResourceAuthority struct {
	serviceID string
	blobStore resourceblob.Store
}

func NewLocalResourceAuthority(serviceID string, blobStore resourceblob.Store) *LocalResourceAuthority {
	return &LocalResourceAuthority{serviceID: serviceID, blobStore: blobStore}
}

func (a *LocalResourceAuthority) OpenResource(ctx context.Context, resourceID string) (ResourceChunkStream, error) {
	reader, info, err := a.blobStore.Open(ctx, resourceID)
	if err != nil {
		return nil, mapResourceBlobError(err)
	}
	return &localResourceStream{
		reader: reader,
		ref: &resourcev1.ResourceRef{
			Id:                 info.ResourceID,
			AuthorityServiceId: a.serviceID,
			Name:               info.Name,
			MimeType:           info.MimeType,
			SizeBytes:          info.SizeBytes,
			ContentHash:        info.ContentHash,
			MetadataJson:       append([]byte(nil), info.MetadataJSON...),
		},
		buf: make([]byte, localAuthorityChunkBytes),
	}, nil
}

type localResourceStream struct {
	reader io.ReadCloser
	ref    *resourcev1.ResourceRef
	buf    []byte
	first  bool
	done   bool
}

func (s *localResourceStream) Recv() (*resourcev1.OpenResourceResponse, error) {
	if s.done {
		return nil, io.EOF
	}
	n, err := s.reader.Read(s.buf)
	if n > 0 {
		resp := &resourcev1.OpenResourceResponse{Data: append([]byte(nil), s.buf[:n]...)}
		if !s.first {
			resp.Resource = s.ref
			s.first = true
		}
		return resp, nil
	}
	if err == io.EOF {
		s.done = true
		_ = s.reader.Close()
		if !s.first {
			s.first = true
			return &resourcev1.OpenResourceResponse{Resource: s.ref}, nil
		}
		return nil, io.EOF
	}
	if err != nil {
		s.done = true
		_ = s.reader.Close()
		return nil, err
	}
	return s.Recv()
}
