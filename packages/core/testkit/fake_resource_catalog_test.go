package testkit

import (
	"context"
	"testing"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
)

func TestFakeResourceCatalogListsResources(t *testing.T) {
	catalog := NewFakeResourceCatalog()
	catalog.AddResource("ws-1", &resourcev1.ResourceRef{Uri: "file:///a.txt", Name: "a.txt"})

	resources, err := catalog.ListResources(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("ListResources returned error: %v", err)
	}
	if len(resources) != 1 || resources[0].GetName() != "a.txt" {
		t.Fatalf("unexpected resources: %+v", resources)
	}
}
