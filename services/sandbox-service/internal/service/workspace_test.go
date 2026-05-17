package service

import (
	"context"
	"testing"

	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	profiledomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/profile"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	workspacestore "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWorkspaceServiceCreateHostedWorkspaceDefaultProfile(t *testing.T) {
	service, backing := newTestWorkspaceService(t)
	resp, err := service.CreateHostedWorkspace(context.Background(), &sandboxv1.CreateHostedWorkspaceRequest{})
	if err != nil {
		t.Fatalf("CreateHostedWorkspace returned error: %v", err)
	}
	ref := resp.GetWorkspace().GetRef()
	if ref.GetServiceId() != "sandbox-service-id" || ref.GetServiceWorkspaceId() == "" || ref.GetSandboxProfileId() != "local-process-dev" {
		t.Fatalf("unexpected workspace ref: %+v", ref)
	}
	if resp.GetWorkspace().GetStatus() != workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE {
		t.Fatalf("unexpected status: %s", resp.GetWorkspace().GetStatus())
	}
	if backing.lastCreate.WorkspaceID != ref.GetServiceWorkspaceId() || backing.lastCreate.SandboxProfileID != "local-process-dev" {
		t.Fatalf("unexpected backing create request: %+v", backing.lastCreate)
	}
}

func TestWorkspaceServiceCreateHostedWorkspaceExplicitProfile(t *testing.T) {
	service, backing := newTestWorkspaceService(t)
	resp, err := service.CreateHostedWorkspace(context.Background(), &sandboxv1.CreateHostedWorkspaceRequest{
		SandboxProfileId: "local-process-dev",
		DisplayName:      "dev workspace",
	})
	if err != nil {
		t.Fatalf("CreateHostedWorkspace returned error: %v", err)
	}
	if got := resp.GetWorkspace().GetRef().GetSandboxProfileId(); got != "local-process-dev" {
		t.Fatalf("unexpected profile: %q", got)
	}
	if got := resp.GetWorkspace().GetDisplayName(); got != "dev workspace" {
		t.Fatalf("unexpected display name: %q", got)
	}
	if backing.lastCreate.SandboxProfileID != "local-process-dev" || backing.lastCreate.DisplayName != "dev workspace" {
		t.Fatalf("unexpected backing create request: %+v", backing.lastCreate)
	}
}

func TestWorkspaceServiceCreateHostedWorkspaceUnknownProfile(t *testing.T) {
	service, _ := newTestWorkspaceService(t)
	_, err := service.CreateHostedWorkspace(context.Background(), &sandboxv1.CreateHostedWorkspaceRequest{
		SandboxProfileId: "missing",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestWorkspaceServiceCreateHostedWorkspaceNoDefaultProfile(t *testing.T) {
	service := NewWorkspaceService(
		"sandbox-service-id",
		profiledomain.NewMemoryRegistry(nil),
		workspacedomain.NewMemoryStore(),
		&fakeBackingStore{},
	)
	_, err := service.CreateHostedWorkspace(context.Background(), &sandboxv1.CreateHostedWorkspaceRequest{})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}
}

func TestWorkspaceServiceCreateHostedWorkspaceDisabledProfile(t *testing.T) {
	service := NewWorkspaceService(
		"sandbox-service-id",
		profiledomain.NewMemoryRegistry([]*profiledomain.Profile{{ID: "disabled", Enabled: false}}),
		workspacedomain.NewMemoryStore(),
		&fakeBackingStore{},
	)
	_, err := service.CreateHostedWorkspace(context.Background(), &sandboxv1.CreateHostedWorkspaceRequest{
		SandboxProfileId: "disabled",
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}
}

func TestWorkspaceServiceGetHostedWorkspace(t *testing.T) {
	service, _ := newTestWorkspaceService(t)
	created, err := service.CreateHostedWorkspace(context.Background(), &sandboxv1.CreateHostedWorkspaceRequest{})
	if err != nil {
		t.Fatalf("CreateHostedWorkspace returned error: %v", err)
	}
	got, err := service.GetHostedWorkspace(context.Background(), &sandboxv1.GetHostedWorkspaceRequest{
		ServiceWorkspaceId: created.GetWorkspace().GetRef().GetServiceWorkspaceId(),
	})
	if err != nil {
		t.Fatalf("GetHostedWorkspace returned error: %v", err)
	}
	if got.GetWorkspace().GetRef().GetServiceWorkspaceId() != created.GetWorkspace().GetRef().GetServiceWorkspaceId() {
		t.Fatalf("unexpected workspace: %+v", got.GetWorkspace())
	}
}

func TestWorkspaceServiceGetHostedWorkspaceEmptyID(t *testing.T) {
	service, _ := newTestWorkspaceService(t)
	_, err := service.GetHostedWorkspace(context.Background(), &sandboxv1.GetHostedWorkspaceRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestWorkspaceServiceGetHostedWorkspaceState(t *testing.T) {
	service, _ := newTestWorkspaceService(t)
	created, err := service.CreateHostedWorkspace(context.Background(), &sandboxv1.CreateHostedWorkspaceRequest{
		SandboxProfileId: "local-process-dev",
	})
	if err != nil {
		t.Fatalf("CreateHostedWorkspace returned error: %v", err)
	}
	stateResp, err := service.GetHostedWorkspaceState(context.Background(), &sandboxv1.GetHostedWorkspaceStateRequest{
		ServiceWorkspaceId: created.GetWorkspace().GetRef().GetServiceWorkspaceId(),
	})
	if err != nil {
		t.Fatalf("GetHostedWorkspaceState returned error: %v", err)
	}
	state := stateResp.GetState()
	if state.GetRef().GetServiceId() != "sandbox-service-id" {
		t.Fatalf("unexpected service id: %q", state.GetRef().GetServiceId())
	}
	if state.GetRef().GetServiceWorkspaceId() != created.GetWorkspace().GetRef().GetServiceWorkspaceId() {
		t.Fatalf("unexpected service workspace id: %q", state.GetRef().GetServiceWorkspaceId())
	}
	if state.GetRef().GetSandboxProfileId() != "local-process-dev" {
		t.Fatalf("unexpected profile id: %q", state.GetRef().GetSandboxProfileId())
	}
	if state.GetStatus() != workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE {
		t.Fatalf("unexpected status: %s", state.GetStatus())
	}
	if state.GetSummary() == "" {
		t.Fatal("expected non-empty summary")
	}
	if len(state.GetFacts()) == 0 {
		t.Fatal("expected facts")
	}
	if state.GetGeneratedAt() == nil {
		t.Fatal("expected generated_at")
	}
}

func TestWorkspaceServiceGetHostedWorkspaceStateEmptyID(t *testing.T) {
	service, _ := newTestWorkspaceService(t)
	_, err := service.GetHostedWorkspaceState(context.Background(), &sandboxv1.GetHostedWorkspaceStateRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestWorkspaceServiceGetHostedWorkspaceStateNotFound(t *testing.T) {
	service, _ := newTestWorkspaceService(t)
	_, err := service.GetHostedWorkspaceState(context.Background(), &sandboxv1.GetHostedWorkspaceStateRequest{
		ServiceWorkspaceId: "missing",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestWorkspaceServiceCreateHostedWorkspaceBackingFailure(t *testing.T) {
	service, backing := newTestWorkspaceService(t)
	backing.createErr = workspacestore.ErrWorkspaceNotReady
	_, err := service.CreateHostedWorkspace(context.Background(), &sandboxv1.CreateHostedWorkspaceRequest{})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}
}

func newTestWorkspaceService(t *testing.T) (*WorkspaceService, *fakeBackingStore) {
	backing := &fakeBackingStore{}
	return NewWorkspaceService(
		"sandbox-service-id",
		testProfileRegistry(),
		workspacedomain.NewMemoryStore(),
		backing,
	), backing
}

func testProfileRegistry() profiledomain.Registry {
	return profiledomain.NewMemoryRegistry([]*profiledomain.Profile{
		{
			ID:             profiledomain.LocalProcessDevID,
			DisplayName:    "Local Process Dev Backend",
			Enabled:        true,
			Default:        true,
			IsolationClass: profiledomain.IsolationDevProcess,
			BackendID:      profiledomain.LocalProcessDevID,
			Capabilities: []profiledomain.Capability{
				profiledomain.CapabilityWorkspaceView,
				profiledomain.CapabilityWorkspaceResource,
				profiledomain.CapabilityWorkspaceExec,
				profiledomain.CapabilityLocalProcessExec,
			},
		},
	})
}
