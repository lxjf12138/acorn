package manifest

import "testing"

func TestDefaultCapabilityService(t *testing.T) {
	service := DefaultCapabilityService()
	if service.GetKind() != "sandbox" {
		t.Fatalf("unexpected kind: %q", service.GetKind())
	}
	if service.GetContract() != "acorn.sandbox" {
		t.Fatalf("unexpected contract: %q", service.GetContract())
	}
}

func TestDefaultProviderManifest(t *testing.T) {
	provider := DefaultProviderManifest()
	if provider.GetAgentSurface().GetProtocol() != "mcp" {
		t.Fatalf("unexpected agent protocol: %q", provider.GetAgentSurface().GetProtocol())
	}
	if !provider.GetSignalSurface().GetSupported() {
		t.Fatal("signal surface is not marked supported")
	}
	if !provider.GetStateSurface().GetSupported() {
		t.Fatal("state surface is not marked supported")
	}
	if !provider.GetGovernanceSurface().GetSupported() {
		t.Fatal("governance surface is not marked supported")
	}
}
