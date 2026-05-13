package testkit

import (
	"context"
	"testing"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	resourcecore "github.com/lxjf12138/acorn/packages/core/resource"
)

func TestFakeResourceCatalogListsResources(t *testing.T) {
	catalog := NewFakeResourceCatalog()
	catalog.AddResource(&resourcev1.ResourceRef{
		Id:        "res-1",
		Uri:       "resource://uploads/a.txt",
		Name:      "a.txt",
		Type:      "file",
		OwnerType: "upload",
		OwnerId:   "upload-1",
	})

	resources, err := catalog.ListResources(context.Background(), resourcecore.ListFilter{
		OwnerType: "upload",
		OwnerID:   "upload-1",
		Type:      "file",
	})
	if err != nil {
		t.Fatalf("ListResources returned error: %v", err)
	}
	if len(resources) != 1 || resources[0].GetName() != "a.txt" {
		t.Fatalf("unexpected resources: %+v", resources)
	}
}
