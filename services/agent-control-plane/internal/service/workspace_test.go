package service

import (
	"context"
	"errors"
	"testing"

	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	sandboxclient "github.com/lxjf12138/acorn/services/agent-control-plane/internal/client/sandbox"
	workspacedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/workspace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeWorkspaceHostClient struct {
	err           error
	stateErr      error
	listErr       error
	previewErr    error
	created       int
	hosted        *sandboxv1.HostedWorkspace
	state         *sandboxv1.HostedWorkspaceState
	listResp      *sandboxv1.ListWorkspaceDirResponse
	previewResp   *sandboxv1.PreviewWorkspaceFileResponse
	lastListInput sandboxclient.ListWorkspaceDirInput
	useHosted     bool
}

func (f *fakeWorkspaceHostClient) CreateHostedWorkspace(_ context.Context, sessionID string, _ string, sandboxProfileID string, _ string) (*sandboxv1.HostedWorkspace, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.created++
	if f.useHosted {
		return f.hosted, nil
	}
	return &sandboxv1.HostedWorkspace{
		Ref: &workspacev1.WorkspaceHostRef{
			ServiceId:          "sandbox-service",
			ServiceWorkspaceId: "ws_" + sessionID,
			SandboxProfileId:   sandboxProfileID,
		},
		Status: workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
	}, nil
}

func (f *fakeWorkspaceHostClient) GetHostedWorkspace(context.Context, string) (*sandboxv1.HostedWorkspace, error) {
	return nil, nil
}

func (f *fakeWorkspaceHostClient) GetHostedWorkspaceState(_ context.Context, _ string, _ string, serviceWorkspaceID string) (*sandboxv1.HostedWorkspaceState, error) {
	if f.stateErr != nil {
		return nil, f.stateErr
	}
	if f.state != nil {
		return f.state, nil
	}
	return &sandboxv1.HostedWorkspaceState{
		Ref: &workspacev1.WorkspaceHostRef{
			ServiceId:          "sandbox-service",
			ServiceWorkspaceId: serviceWorkspaceID,
			SandboxProfileId:   "local-process",
		},
		Status:  workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
		Summary: "empty workspace",
	}, nil
}

func (f *fakeWorkspaceHostClient) ListWorkspaceDir(_ context.Context, input sandboxclient.ListWorkspaceDirInput) (*sandboxv1.ListWorkspaceDirResponse, error) {
	f.lastListInput = input
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.listResp != nil {
		return f.listResp, nil
	}
	return &sandboxv1.ListWorkspaceDirResponse{
		Directory: &sandboxv1.WorkspacePathRef{
			Workspace: &workspacev1.WorkspaceHostRef{
				ServiceId:          "sandbox-service",
				ServiceWorkspaceId: input.ServiceWorkspaceID,
				SandboxProfileId:   "local-process",
			},
			Path: input.Path,
			Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_DIRECTORY,
		},
	}, nil
}

func (f *fakeWorkspaceHostClient) PreviewWorkspaceFile(_ context.Context, input sandboxclient.PreviewWorkspaceFileInput) (*sandboxv1.PreviewWorkspaceFileResponse, error) {
	if f.previewErr != nil {
		return nil, f.previewErr
	}
	if f.previewResp != nil {
		return f.previewResp, nil
	}
	return &sandboxv1.PreviewWorkspaceFileResponse{
		File: &sandboxv1.WorkspacePathRef{
			Workspace: &workspacev1.WorkspaceHostRef{
				ServiceId:          "sandbox-service",
				ServiceWorkspaceId: input.ServiceWorkspaceID,
				SandboxProfileId:   "local-process",
			},
			Path: input.Path,
			Kind: sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE,
		},
		PreviewBytes: []byte("preview"),
	}, nil
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
		hosted *sandboxv1.HostedWorkspace
	}{
		{
			name:   "nil workspace",
			hosted: nil,
		},
		{
			name:   "missing ref",
			hosted: &sandboxv1.HostedWorkspace{},
		},
		{
			name: "missing service id",
			hosted: &sandboxv1.HostedWorkspace{Ref: &workspacev1.WorkspaceHostRef{
				ServiceWorkspaceId: "ws-1",
				SandboxProfileId:   "local-process",
			}},
		},
		{
			name: "missing workspace id",
			hosted: &sandboxv1.HostedWorkspace{Ref: &workspacev1.WorkspaceHostRef{
				ServiceId:        "sandbox-service",
				SandboxProfileId: "local-process",
			}},
		},
		{
			name: "missing profile id",
			hosted: &sandboxv1.HostedWorkspace{Ref: &workspacev1.WorkspaceHostRef{
				ServiceId:          "sandbox-service",
				ServiceWorkspaceId: "ws-1",
			}},
		},
		{
			name: "unexpected service id",
			hosted: &sandboxv1.HostedWorkspace{Ref: &workspacev1.WorkspaceHostRef{
				ServiceId:          "other-service",
				ServiceWorkspaceId: "ws-1",
				SandboxProfileId:   "local-process",
			}},
		},
		{
			name: "unexpected profile id",
			hosted: &sandboxv1.HostedWorkspace{Ref: &workspacev1.WorkspaceHostRef{
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

func TestWorkspaceServiceGetSessionWorkspaceState(t *testing.T) {
	client := &fakeWorkspaceHostClient{}
	service := NewWorkspaceService(workspacedomain.NewMemoryStore(), client, "sandbox-service", "local-process")
	if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}
	got, err := service.GetSessionWorkspaceState(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("GetSessionWorkspaceState returned error: %v", err)
	}
	if got.Record.GetSessionId() != "sess-1" {
		t.Fatalf("unexpected record: %+v", got.Record)
	}
	if got.State.GetRef().GetServiceWorkspaceId() != got.Record.GetCurrentHost().GetServiceWorkspaceId() {
		t.Fatalf("state ref did not match record host: state=%+v record=%+v", got.State.GetRef(), got.Record.GetCurrentHost())
	}
	if got.State.GetSummary() == "" {
		t.Fatal("expected non-empty summary")
	}
}

func TestWorkspaceServiceGetSessionWorkspaceStateMissingSession(t *testing.T) {
	service := NewWorkspaceService(workspacedomain.NewMemoryStore(), &fakeWorkspaceHostClient{}, "sandbox-service", "local-process")
	_, err := service.GetSessionWorkspaceState(context.Background(), "missing")
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestWorkspaceServiceGetSessionWorkspaceStateHostNotFound(t *testing.T) {
	client := &fakeWorkspaceHostClient{stateErr: status.Error(codes.NotFound, "missing")}
	service := NewWorkspaceService(workspacedomain.NewMemoryStore(), client, "sandbox-service", "local-process")
	if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}
	_, err := service.GetSessionWorkspaceState(context.Background(), "sess-1")
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}
}

func TestWorkspaceServiceGetSessionWorkspaceStateRejectsMismatchedRef(t *testing.T) {
	tests := []struct {
		name string
		ref  *workspacev1.WorkspaceHostRef
	}{
		{
			name: "service id",
			ref: &workspacev1.WorkspaceHostRef{
				ServiceId:          "other-service",
				ServiceWorkspaceId: "ws_sess-1",
				SandboxProfileId:   "local-process",
			},
		},
		{
			name: "workspace id",
			ref: &workspacev1.WorkspaceHostRef{
				ServiceId:          "sandbox-service",
				ServiceWorkspaceId: "other-workspace",
				SandboxProfileId:   "local-process",
			},
		},
		{
			name: "profile id",
			ref: &workspacev1.WorkspaceHostRef{
				ServiceId:          "sandbox-service",
				ServiceWorkspaceId: "ws_sess-1",
				SandboxProfileId:   "local-docker",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeWorkspaceHostClient{
				state: &sandboxv1.HostedWorkspaceState{
					Ref:     tt.ref,
					Status:  workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
					Summary: "empty workspace",
				},
			}
			service := NewWorkspaceService(workspacedomain.NewMemoryStore(), client, "sandbox-service", "local-process")
			if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
				t.Fatalf("CreateSessionWorkspace returned error: %v", err)
			}
			if _, err := service.GetSessionWorkspaceState(context.Background(), "sess-1"); status.Code(err) != codes.Internal {
				t.Fatalf("expected Internal, got %v", err)
			}
		})
	}
}

func TestWorkspaceServiceListSessionWorkspaceDir(t *testing.T) {
	client := &fakeWorkspaceHostClient{}
	service := NewWorkspaceService(workspacedomain.NewMemoryStore(), client, "sandbox-service", "local-process")
	if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}
	resp, err := service.ListSessionWorkspaceDir(context.Background(), "sess-1", "user-1", "src", 25, "0")
	if err != nil {
		t.Fatalf("ListSessionWorkspaceDir returned error: %v", err)
	}
	if resp.GetDirectory().GetPath() != "src" {
		t.Fatalf("unexpected directory: %+v", resp.GetDirectory())
	}
	if client.lastListInput.ServiceWorkspaceID != "ws_sess-1" || client.lastListInput.Path != "src" || client.lastListInput.PageSize != 25 || client.lastListInput.PageToken != "0" {
		t.Fatalf("unexpected list input: %+v", client.lastListInput)
	}
}

func TestWorkspaceServiceListSessionWorkspaceDirMissingSession(t *testing.T) {
	service := NewWorkspaceService(workspacedomain.NewMemoryStore(), &fakeWorkspaceHostClient{}, "sandbox-service", "local-process")
	_, err := service.ListSessionWorkspaceDir(context.Background(), "missing", "user-1", "", 0, "")
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestWorkspaceServiceListSessionWorkspaceDirRejectsMismatchedRef(t *testing.T) {
	tests := []struct {
		name string
		resp *sandboxv1.ListWorkspaceDirResponse
	}{
		{
			name: "directory ref",
			resp: &sandboxv1.ListWorkspaceDirResponse{
				Directory: pathRef("other-service", "ws_sess-1", "local-process", ""),
			},
		},
		{
			name: "entry ref",
			resp: &sandboxv1.ListWorkspaceDirResponse{
				Directory: pathRef("sandbox-service", "ws_sess-1", "local-process", ""),
				Entries: []*sandboxv1.WorkspaceDirEntry{
					{Ref: pathRef("sandbox-service", "other-workspace", "local-process", "file.txt")},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeWorkspaceHostClient{listResp: tt.resp}
			service := NewWorkspaceService(workspacedomain.NewMemoryStore(), client, "sandbox-service", "local-process")
			if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
				t.Fatalf("CreateSessionWorkspace returned error: %v", err)
			}
			if _, err := service.ListSessionWorkspaceDir(context.Background(), "sess-1", "user-1", "", 0, ""); status.Code(err) != codes.Internal {
				t.Fatalf("expected Internal, got %v", err)
			}
		})
	}
}

func TestWorkspaceServicePreviewSessionWorkspaceFile(t *testing.T) {
	client := &fakeWorkspaceHostClient{}
	service := NewWorkspaceService(workspacedomain.NewMemoryStore(), client, "sandbox-service", "local-process")
	if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}
	resp, err := service.PreviewSessionWorkspaceFile(context.Background(), "sess-1", "user-1", "report.txt", 64)
	if err != nil {
		t.Fatalf("PreviewSessionWorkspaceFile returned error: %v", err)
	}
	if resp.GetFile().GetPath() != "report.txt" || string(resp.GetPreviewBytes()) != "preview" {
		t.Fatalf("unexpected preview: %+v", resp)
	}
}

func TestWorkspaceServicePreviewSessionWorkspaceFileRejectsMismatchedRef(t *testing.T) {
	client := &fakeWorkspaceHostClient{
		previewResp: &sandboxv1.PreviewWorkspaceFileResponse{
			File: pathRef("sandbox-service", "other-workspace", "local-process", "report.txt"),
		},
	}
	service := NewWorkspaceService(workspacedomain.NewMemoryStore(), client, "sandbox-service", "local-process")
	if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}
	if _, err := service.PreviewSessionWorkspaceFile(context.Background(), "sess-1", "user-1", "report.txt", 0); status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
}

func pathRef(serviceID string, workspaceID string, profileID string, path string) *sandboxv1.WorkspacePathRef {
	return &sandboxv1.WorkspacePathRef{
		Workspace: &workspacev1.WorkspaceHostRef{
			ServiceId:          serviceID,
			ServiceWorkspaceId: workspaceID,
			SandboxProfileId:   profileID,
		},
		Path: path,
	}
}
