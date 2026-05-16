package service

import (
	"context"
	"testing"

	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/descriptor"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWorkspaceServiceCreateHostedWorkspaceDefaultProfile(t *testing.T) {
	service := newTestWorkspaceService()
	resp, err := service.CreateHostedWorkspace(context.Background(), &workspacev1.CreateHostedWorkspaceRequest{})
	if err != nil {
		t.Fatalf("CreateHostedWorkspace returned error: %v", err)
	}
	ref := resp.GetWorkspace().GetRef()
	if ref.GetServiceId() != "sandbox-service" || ref.GetServiceWorkspaceId() == "" || ref.GetSandboxProfileId() != "local-process" {
		t.Fatalf("unexpected workspace ref: %+v", ref)
	}
	if resp.GetWorkspace().GetStatus() != workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE {
		t.Fatalf("unexpected status: %s", resp.GetWorkspace().GetStatus())
	}
}

func TestWorkspaceServiceCreateHostedWorkspaceExplicitProfile(t *testing.T) {
	service := newTestWorkspaceService()
	resp, err := service.CreateHostedWorkspace(context.Background(), &workspacev1.CreateHostedWorkspaceRequest{
		SandboxProfileId: "local-docker",
		DisplayName:      "docker workspace",
	})
	if err != nil {
		t.Fatalf("CreateHostedWorkspace returned error: %v", err)
	}
	if got := resp.GetWorkspace().GetRef().GetSandboxProfileId(); got != "local-docker" {
		t.Fatalf("unexpected profile: %q", got)
	}
	if got := resp.GetWorkspace().GetDisplayName(); got != "docker workspace" {
		t.Fatalf("unexpected display name: %q", got)
	}
}

func TestWorkspaceServiceCreateHostedWorkspaceUnknownProfile(t *testing.T) {
	service := newTestWorkspaceService()
	_, err := service.CreateHostedWorkspace(context.Background(), &workspacev1.CreateHostedWorkspaceRequest{
		SandboxProfileId: "missing",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestWorkspaceServiceGetHostedWorkspace(t *testing.T) {
	service := newTestWorkspaceService()
	created, err := service.CreateHostedWorkspace(context.Background(), &workspacev1.CreateHostedWorkspaceRequest{})
	if err != nil {
		t.Fatalf("CreateHostedWorkspace returned error: %v", err)
	}
	got, err := service.GetHostedWorkspace(context.Background(), &workspacev1.GetHostedWorkspaceRequest{
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
	service := newTestWorkspaceService()
	_, err := service.GetHostedWorkspace(context.Background(), &workspacev1.GetHostedWorkspaceRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestWorkspaceServiceGetHostedWorkspaceState(t *testing.T) {
	service := newTestWorkspaceService()
	created, err := service.CreateHostedWorkspace(context.Background(), &workspacev1.CreateHostedWorkspaceRequest{
		SandboxProfileId: "local-process",
	})
	if err != nil {
		t.Fatalf("CreateHostedWorkspace returned error: %v", err)
	}
	stateResp, err := service.GetHostedWorkspaceState(context.Background(), &workspacev1.GetHostedWorkspaceStateRequest{
		ServiceWorkspaceId: created.GetWorkspace().GetRef().GetServiceWorkspaceId(),
	})
	if err != nil {
		t.Fatalf("GetHostedWorkspaceState returned error: %v", err)
	}
	state := stateResp.GetState()
	if state.GetRef().GetServiceId() != "sandbox-service" {
		t.Fatalf("unexpected service id: %q", state.GetRef().GetServiceId())
	}
	if state.GetRef().GetServiceWorkspaceId() != created.GetWorkspace().GetRef().GetServiceWorkspaceId() {
		t.Fatalf("unexpected service workspace id: %q", state.GetRef().GetServiceWorkspaceId())
	}
	if state.GetRef().GetSandboxProfileId() != "local-process" {
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
	service := newTestWorkspaceService()
	_, err := service.GetHostedWorkspaceState(context.Background(), &workspacev1.GetHostedWorkspaceStateRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestWorkspaceServiceGetHostedWorkspaceStateNotFound(t *testing.T) {
	service := newTestWorkspaceService()
	_, err := service.GetHostedWorkspaceState(context.Background(), &workspacev1.GetHostedWorkspaceStateRequest{
		ServiceWorkspaceId: "missing",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func newTestWorkspaceService() *WorkspaceService {
	return NewWorkspaceService(
		"sandbox-service",
		descriptor.NewSource(descriptor.Options{ServiceID: "sandbox-service"}),
		workspacedomain.NewMemoryStore(),
	)
}
