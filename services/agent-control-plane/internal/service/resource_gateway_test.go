package service

import (
	"context"
	"io"
	"strings"
	"testing"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	"github.com/lxjf12138/acorn/packages/core/resourceblob"
	"github.com/lxjf12138/acorn/packages/servicekit/localblob"
	resourcedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestResourceGatewayServiceOpenResource(t *testing.T) {
	store := resourcedomain.NewMemoryStore()
	record := registerGatewayResource(t, store, "res_1", "user-1", resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE, "sandbox-service")
	authority := &fakeAuthorityClient{stream: &fakeResourceContentClient{chunks: []*resourcev1.OpenResourceResponse{{Resource: record.GetRef(), Data: []byte("data")}}}}
	gateway := NewResourceGatewayService(store, map[string]ResourceAuthorityClient{"sandbox-service": authority})

	got, stream, err := gateway.OpenResource(context.Background(), "res_1", "user-1")
	if err != nil {
		t.Fatalf("OpenResource returned error: %v", err)
	}
	if got.GetRef().GetId() != record.GetRef().GetId() {
		t.Fatalf("unexpected record: %+v", got)
	}
	if stream == nil {
		t.Fatal("expected stream")
	}
	if authority.openedID != "res_1" {
		t.Fatalf("unexpected opened id: %q", authority.openedID)
	}
}

func TestResourceGatewayServiceOpenResourceForTransferLocalAuthority(t *testing.T) {
	resourceStore := resourcedomain.NewMemoryStore()
	blobStore, err := localblob.NewStore(localblob.Config{BaseDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	blob, err := blobStore.Put(context.Background(), resourceblob.PutRequest{
		ResourceID: "res_local",
		Name:       "upload.txt",
		MimeType:   "text/plain",
		Source:     strings.NewReader("uploaded"),
	})
	if err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	if _, err := resourceStore.Register(context.Background(), &resourcev1.ResourceRecord{
		Ref: &resourcev1.ResourceRef{
			Id:                 blob.ResourceID,
			AuthorityServiceId: "agent-control-plane",
			Name:               blob.Name,
			MimeType:           blob.MimeType,
			SizeBytes:          blob.SizeBytes,
			ContentHash:        blob.ContentHash,
		},
		OwnerUserId: "user-1",
		Status:      resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE,
		Visibility:  resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE,
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	gateway := NewResourceGatewayService(resourceStore, map[string]ResourceAuthorityClient{
		"agent-control-plane": NewLocalResourceAuthority("agent-control-plane", blobStore),
	})

	stream, err := gateway.OpenResourceForTransfer(context.Background(), "res_local", "user-1")
	if err != nil {
		t.Fatalf("OpenResourceForTransfer returned error: %v", err)
	}
	body, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read resource stream: %v", err)
	}
	if string(body) != "uploaded" {
		t.Fatalf("unexpected local authority body: %q", string(body))
	}
}

func TestResourceGatewayServiceOpenResourceErrors(t *testing.T) {
	tests := []struct {
		name       string
		resourceID string
		userID     string
		setup      func(*testing.T, *resourcedomain.MemoryStore) map[string]ResourceAuthorityClient
		code       codes.Code
	}{
		{
			name: "missing resource id",
			code: codes.InvalidArgument,
		},
		{
			name:       "missing resource",
			resourceID: "missing",
			code:       codes.NotFound,
		},
		{
			name:       "unavailable resource",
			resourceID: "res_unavailable",
			userID:     "user-1",
			setup: func(t *testing.T, store *resourcedomain.MemoryStore) map[string]ResourceAuthorityClient {
				registerGatewayResource(t, store, "res_unavailable", "user-1", resourcev1.ResourceStatus_RESOURCE_STATUS_UNAVAILABLE, "sandbox-service")
				return map[string]ResourceAuthorityClient{"sandbox-service": &fakeAuthorityClient{}}
			},
			code: codes.FailedPrecondition,
		},
		{
			name:       "owner mismatch",
			resourceID: "res_owner",
			userID:     "user-2",
			setup: func(t *testing.T, store *resourcedomain.MemoryStore) map[string]ResourceAuthorityClient {
				registerGatewayResource(t, store, "res_owner", "user-1", resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE, "sandbox-service")
				return map[string]ResourceAuthorityClient{"sandbox-service": &fakeAuthorityClient{}}
			},
			code: codes.PermissionDenied,
		},
		{
			name:       "unsupported authority",
			resourceID: "res_authority",
			userID:     "user-1",
			setup: func(t *testing.T, store *resourcedomain.MemoryStore) map[string]ResourceAuthorityClient {
				registerGatewayResource(t, store, "res_authority", "user-1", resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE, "other-service")
				return map[string]ResourceAuthorityClient{"sandbox-service": &fakeAuthorityClient{}}
			},
			code: codes.FailedPrecondition,
		},
		{
			name:       "authority open error",
			resourceID: "res_open",
			userID:     "user-1",
			setup: func(t *testing.T, store *resourcedomain.MemoryStore) map[string]ResourceAuthorityClient {
				registerGatewayResource(t, store, "res_open", "user-1", resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE, "sandbox-service")
				return map[string]ResourceAuthorityClient{"sandbox-service": &fakeAuthorityClient{err: status.Error(codes.Unavailable, "sandbox unavailable")}}
			},
			code: codes.Unavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := resourcedomain.NewMemoryStore()
			authorities := map[string]ResourceAuthorityClient{}
			if tt.setup != nil {
				authorities = tt.setup(t, store)
			}
			gateway := NewResourceGatewayService(store, authorities)
			_, _, err := gateway.OpenResource(context.Background(), tt.resourceID, tt.userID)
			if status.Code(err) != tt.code {
				t.Fatalf("expected %s, got %v", tt.code, err)
			}
		})
	}
}

func registerGatewayResource(t *testing.T, store *resourcedomain.MemoryStore, resourceID string, owner string, resourceStatus resourcev1.ResourceStatus, authority string) *resourcev1.ResourceRecord {
	t.Helper()
	record, err := store.Register(context.Background(), &resourcev1.ResourceRecord{
		Ref: &resourcev1.ResourceRef{
			Id:                 resourceID,
			AuthorityServiceId: authority,
			Name:               "report.txt",
			MimeType:           "text/plain",
			SizeBytes:          6,
			ContentHash:        "sha256:abc",
		},
		OwnerUserId: owner,
		Status:      resourceStatus,
		Visibility:  resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE,
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	return record
}

type fakeAuthorityClient struct {
	stream   ResourceChunkStream
	err      error
	openedID string
}

func (f *fakeAuthorityClient) OpenResource(_ context.Context, resourceID string) (ResourceChunkStream, error) {
	f.openedID = resourceID
	if f.err != nil {
		return nil, f.err
	}
	return f.stream, nil
}

type fakeResourceContentClient struct {
	ResourceChunkStream
	chunks []*resourcev1.OpenResourceResponse
	index  int
}

func (f *fakeResourceContentClient) Recv() (*resourcev1.OpenResourceResponse, error) {
	if f.index >= len(f.chunks) {
		return nil, io.EOF
	}
	chunk := f.chunks[f.index]
	f.index++
	return chunk, nil
}

func (f *fakeResourceContentClient) CloseSend() error {
	return nil
}
