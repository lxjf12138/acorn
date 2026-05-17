package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	resourceblob "github.com/lxjf12138/acorn/packages/core/resourceblob"
	exporteddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/exportedresource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestResourceContentServiceOpenResourceStreamsBlob(t *testing.T) {
	exportStore := exporteddomain.NewMemoryStore()
	if _, err := exportStore.Create(context.Background(), exporteddomain.Record{
		ResourceID:               "res_1",
		BlobStoreKind:            "fakeblob",
		BlobID:                   "blob_1",
		Name:                     "report.txt",
		MimeType:                 "text/plain",
		SizeBytes:                11,
		ContentHash:              "sha256:abc",
		SourceServiceWorkspaceID: "ws_1",
		SourceWorkspacePath:      "outputs/report.txt",
	}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	blobStore := &streamingBlobStore{
		info: &resourceblob.BlobInfo{
			ResourceID:   "res_1",
			StoreKind:    "fakeblob",
			StoreBlobID:  "blob_1",
			Name:         "report.txt",
			MimeType:     "text/plain",
			SizeBytes:    11,
			ContentHash:  "sha256:abc",
			MetadataJSON: []byte(`{"k":"v"}`),
		},
		content: "hello world",
	}
	service := NewResourceContentService("sandbox-service", exportStore, blobStore)
	service.chunkBytes = 5
	stream := &fakeOpenResourceServer{ctx: context.Background()}

	err := service.OpenResource(&resourcev1.OpenResourceRequest{ResourceId: "res_1"}, stream)
	if err != nil {
		t.Fatalf("OpenResource returned error: %v", err)
	}
	if blobStore.openedID != "blob_1" {
		t.Fatalf("expected blob id, got %q", blobStore.openedID)
	}
	if len(stream.chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(stream.chunks))
	}
	first := stream.chunks[0]
	if first.GetResource().GetId() != "res_1" ||
		first.GetResource().GetAuthorityServiceId() != "sandbox-service" ||
		first.GetResource().GetName() != "report.txt" ||
		first.GetResource().GetMimeType() != "text/plain" ||
		first.GetResource().GetSizeBytes() != 11 ||
		first.GetResource().GetContentHash() != "sha256:abc" ||
		string(first.GetResource().GetMetadataJson()) != `{"k":"v"}` {
		t.Fatalf("unexpected first resource metadata: %+v", first.GetResource())
	}
	if stream.chunks[1].GetResource() != nil || stream.chunks[2].GetResource() != nil {
		t.Fatalf("expected only first chunk to carry resource metadata: %+v", stream.chunks)
	}
	if got := string(stream.chunks[0].GetData()) + string(stream.chunks[1].GetData()) + string(stream.chunks[2].GetData()); got != "hello world" {
		t.Fatalf("unexpected streamed content: %q", got)
	}
}

func TestResourceContentServiceOpenResourceEmptyBlobSendsMetadata(t *testing.T) {
	exportStore := exporteddomain.NewMemoryStore()
	if _, err := exportStore.Create(context.Background(), exporteddomain.Record{ResourceID: "res_empty", BlobID: "res_empty", Name: "empty.txt"}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	service := NewResourceContentService("sandbox-service", exportStore, &streamingBlobStore{
		info: &resourceblob.BlobInfo{ResourceID: "res_empty", StoreBlobID: "res_empty", Name: "empty.txt"},
	})
	stream := &fakeOpenResourceServer{ctx: context.Background()}

	err := service.OpenResource(&resourcev1.OpenResourceRequest{ResourceId: "res_empty"}, stream)
	if err != nil {
		t.Fatalf("OpenResource returned error: %v", err)
	}
	if len(stream.chunks) != 1 || stream.chunks[0].GetResource().GetId() != "res_empty" || len(stream.chunks[0].GetData()) != 0 {
		t.Fatalf("unexpected chunks: %+v", stream.chunks)
	}
}

func TestResourceContentServiceOpenResourceErrors(t *testing.T) {
	tests := []struct {
		name        string
		resourceID  string
		exportStore exporteddomain.Store
		blobStore   resourceblob.Store
		code        codes.Code
	}{
		{
			name:        "missing resource id",
			exportStore: exporteddomain.NewMemoryStore(),
			blobStore:   &streamingBlobStore{},
			code:        codes.InvalidArgument,
		},
		{
			name:        "missing export record",
			resourceID:  "missing",
			exportStore: exporteddomain.NewMemoryStore(),
			blobStore:   &streamingBlobStore{},
			code:        codes.NotFound,
		},
		{
			name:       "missing blob",
			resourceID: "res_missing_blob",
			exportStore: exportStoreWithRecord(t, exporteddomain.Record{
				ResourceID: "res_missing_blob",
				BlobID:     "missing_blob",
			}),
			blobStore: &streamingBlobStore{openErr: resourceblob.ErrNotFound},
			code:      codes.NotFound,
		},
		{
			name:       "reader error",
			resourceID: "res_reader_error",
			exportStore: exportStoreWithRecord(t, exporteddomain.Record{
				ResourceID: "res_reader_error",
				BlobID:     "blob_reader_error",
			}),
			blobStore: &streamingBlobStore{
				info:   &resourceblob.BlobInfo{ResourceID: "res_reader_error", StoreBlobID: "blob_reader_error"},
				reader: errReader{},
			},
			code: codes.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewResourceContentService("sandbox-service", tt.exportStore, tt.blobStore)
			err := service.OpenResource(&resourcev1.OpenResourceRequest{ResourceId: tt.resourceID}, &fakeOpenResourceServer{ctx: context.Background()})
			if status.Code(err) != tt.code {
				t.Fatalf("expected %s, got %v", tt.code, err)
			}
		})
	}
}

func exportStoreWithRecord(t *testing.T, record exporteddomain.Record) exporteddomain.Store {
	t.Helper()
	store := exporteddomain.NewMemoryStore()
	if _, err := store.Create(context.Background(), record); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	return store
}

type streamingBlobStore struct {
	info     *resourceblob.BlobInfo
	content  string
	reader   io.Reader
	openErr  error
	openedID string
}

func (s *streamingBlobStore) Kind() string { return "fakeblob" }

func (s *streamingBlobStore) Put(context.Context, resourceblob.PutRequest) (*resourceblob.StoredBlob, error) {
	return nil, errors.New("not implemented")
}

func (s *streamingBlobStore) Open(_ context.Context, resourceID string) (io.ReadCloser, *resourceblob.BlobInfo, error) {
	s.openedID = resourceID
	if s.openErr != nil {
		return nil, nil, s.openErr
	}
	info := *s.info
	reader := s.reader
	if reader == nil {
		reader = strings.NewReader(s.content)
	}
	return io.NopCloser(reader), &info, nil
}

func (s *streamingBlobStore) Stat(context.Context, string) (*resourceblob.BlobInfo, error) {
	return nil, errors.New("not implemented")
}

func (s *streamingBlobStore) Delete(context.Context, string) error {
	return errors.New("not implemented")
}

type fakeOpenResourceServer struct {
	resourcev1.ResourceContentService_OpenResourceServer
	ctx    context.Context
	chunks []*resourcev1.OpenResourceResponse
}

func (s *fakeOpenResourceServer) Context() context.Context {
	return s.ctx
}

func (s *fakeOpenResourceServer) Send(chunk *resourcev1.OpenResourceResponse) error {
	s.chunks = append(s.chunks, chunk)
	return nil
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}
