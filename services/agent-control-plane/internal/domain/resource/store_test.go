package resource

import (
	"context"
	"testing"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMemoryStoreRegisterFillsDefaults(t *testing.T) {
	store := NewMemoryStore()
	record, err := store.Register(context.Background(), &resourcev1.ResourceRecord{
		Ref: &resourcev1.ResourceRef{
			AuthorityServiceId: "resource-store",
			Name:               "report.pdf",
			MimeType:           "application/pdf",
		},
		OwnerUserId: "user-1",
		SessionId:   "session-1",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if record.GetRef().GetId() == "" {
		t.Fatal("Register did not assign resource id")
	}
	if record.GetStatus() != resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE {
		t.Fatalf("unexpected status: %v", record.GetStatus())
	}
	if record.GetVisibility() != resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_INTERNAL {
		t.Fatalf("unexpected visibility: %v", record.GetVisibility())
	}
	if record.GetCreatedAt() == nil || record.GetUpdatedAt() == nil {
		t.Fatalf("timestamps were not filled: %+v", record)
	}
}

func TestMemoryStoreRegisterRejectsInvalidRecord(t *testing.T) {
	store := NewMemoryStore()
	_, err := store.Register(context.Background(), &resourcev1.ResourceRecord{
		Ref: &resourcev1.ResourceRef{Name: "report.pdf"},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestMemoryStoreRegisterRejectsDuplicateID(t *testing.T) {
	store := NewMemoryStore()
	record := &resourcev1.ResourceRecord{
		Ref: &resourcev1.ResourceRef{
			Id:                 "res-1",
			AuthorityServiceId: "resource-store",
			Name:               "report.pdf",
		},
	}
	if _, err := store.Register(context.Background(), record); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if _, err := store.Register(context.Background(), record); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v", err)
	}
}

func TestMemoryStoreGetMissing(t *testing.T) {
	store := NewMemoryStore()
	if _, err := store.Get(context.Background(), "missing"); status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestMemoryStoreListFilters(t *testing.T) {
	store := NewMemoryStore()
	register := func(id string, owner string, session string, visibility resourcev1.ResourceVisibility) {
		t.Helper()
		if _, err := store.Register(context.Background(), &resourcev1.ResourceRecord{
			Ref: &resourcev1.ResourceRef{
				Id:                 id,
				AuthorityServiceId: "resource-store",
				Name:               id + ".txt",
			},
			OwnerUserId: owner,
			SessionId:   session,
			Visibility:  visibility,
		}); err != nil {
			t.Fatalf("Register returned error: %v", err)
		}
	}
	register("res-1", "user-1", "session-1", resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE)
	register("res-2", "user-1", "session-2", resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_INTERNAL)
	register("res-3", "user-2", "session-1", resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE)

	resources, err := store.List(context.Background(), Filter{
		OwnerUserID: "user-1",
		SessionID:   "session-1",
		Visibility:  resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE,
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(resources) != 1 || resources[0].GetRef().GetId() != "res-1" {
		t.Fatalf("unexpected resources: %+v", resources)
	}
}
