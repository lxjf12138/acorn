package service

import (
	"context"
	"errors"
	"testing"

	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	workspacedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/workspace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeWorkspaceHostClient struct {
	err       error
	created   int
	hosted    *workspacev1.HostedWorkspace
	useHosted bool
}

func (f *fakeWorkspaceHostClient) CreateHostedWorkspace(_ context.Context, sessionID string, _ string, sandboxProfileID string, _ string) (*workspacev1.HostedWorkspace, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.created++
	if f.useHosted {
		return f.hosted, nil
	}
	return &workspacev1.HostedWorkspace{
		Ref: &workspacev1.WorkspaceHostRef{
			ServiceId:          "sandbox-service",
			ServiceWorkspaceId: "ws_" + sessionID,
			SandboxProfileId:   sandboxProfileID,
		},
		Status: workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
	}, nil
}

func (f *fakeWorkspaceHostClient) GetHostedWorkspace(context.Context, string) (*workspacev1.HostedWorkspace, error) {
	return nil, nil
}

func (f *fakeWorkspaceHostClient) Close() error { return nil }

func TestWorkspaceServiceCreateSessionWorkspace(t *testing.T) {
	client := &fakeWorkspaceHostClient{}
	service := NewWorkspaceService(workspacedomain.NewMemoryStore(), client, "sandbox-service", "local-process")
	record, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1")
	if err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}
	if record.GetSessionId() != "sess-1" || record.GetCurrentHost().GetServiceWorkspaceId() != "ws_sess-1" {
		t.Fatalf("unexpected record: %+v", record)
	}
	if client.created != 1 {
		t.Fatalf("unexpected hosted workspace create count: %d", client.created)
	}
}

func TestWorkspaceServiceCreateSessionWorkspaceIdempotent(t *testing.T) {
	client := &fakeWorkspaceHostClient{}
	service := NewWorkspaceService(workspacedomain.NewMemoryStore(), client, "sandbox-service", "local-process")
	first, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1")
	if err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}
	second, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1")
	if err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}
	if first.GetId() != second.GetId() || first.GetCurrentHost().GetServiceWorkspaceId() != second.GetCurrentHost().GetServiceWorkspaceId() {
		t.Fatalf("expected idempotent record: first=%+v second=%+v", first, second)
	}
	if client.created != 1 {
		t.Fatalf("expected one hosted workspace, got %d", client.created)
	}
}

func TestWorkspaceServiceDifferentSessionsCreateDifferentRecords(t *testing.T) {
	client := &fakeWorkspaceHostClient{}
	service := NewWorkspaceService(workspacedomain.NewMemoryStore(), client, "sandbox-service", "local-process")
	first, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1")
	if err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}
	second, err := service.CreateSessionWorkspace(context.Background(), "sess-2", "user-1")
	if err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}
	if first.GetId() == second.GetId() || first.GetCurrentHost().GetServiceWorkspaceId() == second.GetCurrentHost().GetServiceWorkspaceId() {
		t.Fatalf("different sessions reused workspace: first=%+v second=%+v", first, second)
	}
}

func TestWorkspaceServiceSandboxFailureDoesNotCreateRecord(t *testing.T) {
	client := &fakeWorkspaceHostClient{err: errors.New("sandbox unavailable")}
	store := workspacedomain.NewMemoryStore()
	service := NewWorkspaceService(store, client, "sandbox-service", "local-process")
	if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err == nil {
		t.Fatal("expected error")
	}
	if _, ok, err := store.GetBySession(context.Background(), "sess-1"); err != nil || ok {
		t.Fatalf("unexpected stored record after sandbox failure: ok=%v err=%v", ok, err)
	}
}

func TestWorkspaceServiceRejectsInvalidHostedWorkspace(t *testing.T) {
	tests := []struct {
		name   string
		hosted *workspacev1.HostedWorkspace
	}{
		{
			name:   "nil workspace",
			hosted: nil,
		},
		{
			name:   "missing ref",
			hosted: &workspacev1.HostedWorkspace{},
		},
		{
			name: "missing service id",
			hosted: &workspacev1.HostedWorkspace{Ref: &workspacev1.WorkspaceHostRef{
				ServiceWorkspaceId: "ws-1",
				SandboxProfileId:   "local-process",
			}},
		},
		{
			name: "missing workspace id",
			hosted: &workspacev1.HostedWorkspace{Ref: &workspacev1.WorkspaceHostRef{
				ServiceId:        "sandbox-service",
				SandboxProfileId: "local-process",
			}},
		},
		{
			name: "missing profile id",
			hosted: &workspacev1.HostedWorkspace{Ref: &workspacev1.WorkspaceHostRef{
				ServiceId:          "sandbox-service",
				ServiceWorkspaceId: "ws-1",
			}},
		},
		{
			name: "unexpected service id",
			hosted: &workspacev1.HostedWorkspace{Ref: &workspacev1.WorkspaceHostRef{
				ServiceId:          "other-service",
				ServiceWorkspaceId: "ws-1",
				SandboxProfileId:   "local-process",
			}},
		},
		{
			name: "unexpected profile id",
			hosted: &workspacev1.HostedWorkspace{Ref: &workspacev1.WorkspaceHostRef{
				ServiceId:          "sandbox-service",
				ServiceWorkspaceId: "ws-1",
				SandboxProfileId:   "local-docker",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeWorkspaceHostClient{hosted: tt.hosted, useHosted: true}
			store := workspacedomain.NewMemoryStore()
			service := NewWorkspaceService(store, client, "sandbox-service", "local-process")
			if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); status.Code(err) != codes.Internal {
				t.Fatalf("expected Internal, got %v", err)
			}
			if _, ok, err := store.GetBySession(context.Background(), "sess-1"); err != nil || ok {
				t.Fatalf("unexpected stored record after invalid hosted workspace: ok=%v err=%v", ok, err)
			}
		})
	}
}
