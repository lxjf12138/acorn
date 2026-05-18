package service

import (
	"context"
	"io"
	"strings"
	"testing"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	"github.com/lxjf12138/acorn/packages/core/events"
	resourceblob "github.com/lxjf12138/acorn/packages/core/resourceblob"
	"github.com/lxjf12138/acorn/packages/servicekit/localblob"
	resourcedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUploadServiceUploadResource(t *testing.T) {
	blobStore := newUploadTestBlobStore(t)
	resourceService := NewResourceService(resourcedomain.NewMemoryStore())
	emitter := &fakeEventEmitter{}
	upload := NewUploadServiceWithEvents("agent-control-plane", blobStore, resourceService, 100, emitter)

	record, err := upload.UploadResource(context.Background(), UploadResourceInput{
		UserID:    "user-1",
		SessionID: "sess-1",
		Name:      "../data.csv",
		MimeType:  "text/csv",
		Source:    strings.NewReader("hello"),
	})
	if err != nil {
		t.Fatalf("UploadResource returned error: %v", err)
	}
	ref := record.GetRef()
	if ref.GetId() == "" ||
		ref.GetAuthorityServiceId() != "agent-control-plane" ||
		ref.GetName() != ".._data.csv" ||
		ref.GetMimeType() != "text/csv" ||
		ref.GetSizeBytes() != 5 ||
		ref.GetContentHash() != "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("unexpected uploaded ref: %+v", ref)
	}
	if record.GetOwnerUserId() != "user-1" ||
		record.GetSessionId() != "sess-1" ||
		record.GetVisibility() != resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE ||
		record.GetSource().GetType() != "user_upload" ||
		record.GetSource().GetSourceServiceId() != "agent-control-plane" {
		t.Fatalf("unexpected uploaded record: %+v", record)
	}
	reader, _, err := blobStore.Open(context.Background(), ref.GetId())
	if err != nil {
		t.Fatalf("Open uploaded blob returned error: %v", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read uploaded blob: %v", err)
	}
	if string(body) != "hello" {
		t.Fatalf("unexpected blob body: %q", string(body))
	}
	if len(emitter.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(emitter.events))
	}
	event := emitter.events[0]
	if event.Name != events.ResourceUploaded || event.Severity != events.SeverityInfo {
		t.Fatalf("unexpected event: %+v", event)
	}
	if event.Attributes[events.AttrResourceAuthorityServiceID] != "agent-control-plane" ||
		event.Attributes[events.AttrResourceMimeType] != "text/csv" ||
		event.Attributes[events.AttrResourceSizeBytes] != int64(5) {
		t.Fatalf("unexpected event attributes: %+v", event.Attributes)
	}
}

func TestUploadServiceErrors(t *testing.T) {
	upload := NewUploadService("agent-control-plane", newUploadTestBlobStore(t), NewResourceService(resourcedomain.NewMemoryStore()), 4)
	if _, err := upload.UploadResource(context.Background(), UploadResourceInput{Name: "x.txt", Source: strings.NewReader("x")}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected missing user InvalidArgument, got %v", err)
	}
	if _, err := upload.UploadResource(context.Background(), UploadResourceInput{UserID: "user-1", Name: "x.txt"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected missing source InvalidArgument, got %v", err)
	}
	if _, err := upload.UploadResource(context.Background(), UploadResourceInput{UserID: "user-1", Name: "x.txt", Source: strings.NewReader("large")}); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v", err)
	}
}

func TestUploadServiceRollsBackBlobWithFakeStore(t *testing.T) {
	blobStore := &rollbackBlobStore{}
	upload := NewUploadService("agent-control-plane", blobStore, failingRegistrar{}, 100)
	_, err := upload.UploadResource(context.Background(), UploadResourceInput{
		UserID: "user-1",
		Name:   "report.txt",
		Source: strings.NewReader("hello"),
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
	if !blobStore.deleted {
		t.Fatal("expected rollback delete")
	}
}

func newUploadTestBlobStore(t *testing.T) *localblob.Store {
	t.Helper()
	store, err := localblob.NewStore(localblob.Config{BaseDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	return store
}

type failingRegistrar struct{}

func (failingRegistrar) RegisterRecord(context.Context, *resourcev1.RegisterResourceRequest) (*resourcev1.ResourceRecord, error) {
	return nil, status.Error(codes.Internal, "register failed")
}

type rollbackBlobStore struct {
	deleted bool
}

type fakeEventEmitter struct {
	events []events.Event
}

func (e *fakeEventEmitter) Emit(_ context.Context, event events.Event) {
	e.events = append(e.events, event)
}

func (s *rollbackBlobStore) Kind() string { return "fake" }

func (s *rollbackBlobStore) Put(_ context.Context, req resourceblob.PutRequest) (*resourceblob.StoredBlob, error) {
	if req.Source != nil {
		_, _ = io.ReadAll(req.Source)
	}
	return &resourceblob.StoredBlob{ResourceID: req.ResourceID, StoreKind: "fake", StoreBlobID: req.ResourceID, Name: req.Name}, nil
}

func (s *rollbackBlobStore) Open(context.Context, string) (io.ReadCloser, *resourceblob.BlobInfo, error) {
	return nil, nil, resourceblob.ErrNotFound
}

func (s *rollbackBlobStore) Stat(context.Context, string) (*resourceblob.BlobInfo, error) {
	return nil, resourceblob.ErrNotFound
}

func (s *rollbackBlobStore) Delete(context.Context, string) error {
	s.deleted = true
	return nil
}
