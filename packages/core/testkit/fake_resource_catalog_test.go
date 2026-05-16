package testkit

import (
	"context"
	"errors"
	"testing"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	resourcecore "github.com/lxjf12138/acorn/packages/core/resource"
)

func TestFakeResourceCatalogRegistersGetsAndListsResources(t *testing.T) {
	catalog := NewFakeResourceCatalog()
	record, err := catalog.Register(context.Background(), &resourcev1.ResourceRecord{
		Ref: &resourcev1.ResourceRef{
			AuthorityServiceId: "resource-store",
			Name:               "a.txt",
			MimeType:           "text/plain",
			SizeBytes:          12,
		},
		OwnerUserId: "user-1",
		SessionId:   "session-1",
		Status:      resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE,
		Visibility:  resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE,
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if record.GetRef().GetId() == "" || record.GetRef().GetName() != "a.txt" {
		t.Fatalf("unexpected registered resource: %+v", record)
	}
	if record.GetStatus() != resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE {
		t.Fatalf("unexpected status: %v", record.GetStatus())
	}
	if record.GetVisibility() != resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE {
		t.Fatalf("unexpected visibility: %v", record.GetVisibility())
	}
	if record.GetCreatedAt() == nil || record.GetUpdatedAt() == nil {
		t.Fatalf("timestamps were not filled: %+v", record)
	}

	got, err := catalog.Get(context.Background(), record.GetRef().GetId())
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.GetOwnerUserId() != "user-1" {
		t.Fatalf("unexpected resource: %+v", got)
	}

	resources, err := catalog.List(context.Background(), resourcecore.ListFilter{
		OwnerUserID: "user-1",
		SessionID:   "session-1",
		Status:      resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE,
		Visibility:  resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE,
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(resources) != 1 || resources[0].GetRef().GetName() != "a.txt" {
		t.Fatalf("unexpected resources: %+v", resources)
	}
}

func TestFakeResourceCatalogRejectsDuplicateIDs(t *testing.T) {
	catalog := NewFakeResourceCatalog()
	record := &resourcev1.ResourceRecord{
		Ref: &resourcev1.ResourceRef{
			Id:                 "res-1",
			AuthorityServiceId: "resource-store",
			Name:               "a.txt",
		},
	}
	if _, err := catalog.Register(context.Background(), record); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if _, err := catalog.Register(context.Background(), record); !errors.Is(err, resourcecore.ErrAlreadyExists) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestFakeResourceCatalogReturnsNotFound(t *testing.T) {
	catalog := NewFakeResourceCatalog()
	if _, err := catalog.Get(context.Background(), "missing"); !errors.Is(err, resourcecore.ErrResourceNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
}
