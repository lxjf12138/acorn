package testkit

import (
	"context"
	"testing"

	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
)

func TestFakeCapabilityCatalogListsServices(t *testing.T) {
	catalog := NewFakeCapabilityCatalog()
	catalog.AddService(&capabilityv1.CapabilityService{
		Id:       "sandbox.local.1",
		Name:     "Sandbox Service",
		Kind:     "sandbox",
		Contract: "acorn.sandbox",
		Features: []string{"workspaces"},
	})

	services, err := catalog.ListServices(context.Background())
	if err != nil {
		t.Fatalf("ListServices returned error: %v", err)
	}
	if len(services) != 1 || services[0].GetId() != "sandbox.local.1" {
		t.Fatalf("unexpected services: %+v", services)
	}
	if services[0].GetContract() != "acorn.sandbox" {
		t.Fatalf("unexpected contract: %q", services[0].GetContract())
	}
	if got := services[0].GetFeatures(); len(got) != 1 || got[0] != "workspaces" {
		t.Fatalf("unexpected features: %v", got)
	}
}
