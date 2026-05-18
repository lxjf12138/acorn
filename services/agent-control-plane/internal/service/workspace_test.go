package service

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	sandboxclient "github.com/lxjf12138/acorn/services/agent-control-plane/internal/client/sandbox"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/conf"
	eventdomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/event"
	resourcedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/resource"
	sandboxpolicydomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/sandboxpolicy"
	workspacedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/workspace"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/infra/eventstore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeWorkspaceHostClient struct {
	err                   error
	stateErr              error
	listErr               error
	previewErr            error
	exportErr             error
	created               int
	hosted                *sandboxv1.HostedWorkspace
	state                 *sandboxv1.HostedWorkspaceState
	listResp              *sandboxv1.ListWorkspaceDirResponse
	previewResp           *sandboxv1.PreviewWorkspaceFileResponse
	exportResp            *sandboxv1.ExportWorkspacePathResponse
	importResp            *sandboxv1.ImportResourceToWorkspaceResponse
	importErr             error
	execResp              *sandboxv1.ExecWorkspaceCommandResponse
	execErr               error
	descriptor            *capabilityv1.CapabilityDescriptor
	descriptorErr         error
	descriptorCalls       int
	lastCreateSessionID   string
	lastCreateOwnerUserID string
	lastCreateProfileID   string
	lastListInput         sandboxclient.ListWorkspaceDirInput
	lastExportInput       sandboxclient.ExportWorkspacePathInput
	lastImportInput       sandboxclient.ImportResourceInput
	lastExecInput         sandboxclient.ExecWorkspaceCommandInput
	lastImportBody        string
	useHosted             bool
}

func (f *fakeWorkspaceHostClient) GetCapabilityDescriptor(context.Context) (*capabilityv1.CapabilityDescriptor, error) {
	f.descriptorCalls++
	if f.descriptorErr != nil {
		return nil, f.descriptorErr
	}
	if f.descriptor != nil {
		return f.descriptor, nil
	}
	return sandboxDescriptor("local-process"), nil
}

func (f *fakeWorkspaceHostClient) CreateHostedWorkspace(_ context.Context, sessionID string, ownerUserID string, sandboxProfileID string, _ string) (*sandboxv1.HostedWorkspace, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.created++
	f.lastCreateSessionID = sessionID
	f.lastCreateOwnerUserID = ownerUserID
	f.lastCreateProfileID = sandboxProfileID
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

func (f *fakeWorkspaceHostClient) ExportWorkspacePath(_ context.Context, input sandboxclient.ExportWorkspacePathInput) (*sandboxv1.ExportWorkspacePathResponse, error) {
	f.lastExportInput = input
	if f.exportErr != nil {
		return nil, f.exportErr
	}
	if f.exportResp != nil {
		return f.exportResp, nil
	}
	return &sandboxv1.ExportWorkspacePathResponse{
		Source: pathRef("sandbox-service", input.ServiceWorkspaceID, "local-process", input.Path),
		Resource: &resourcev1.ResourceRef{
			Id:                 "res_1",
			AuthorityServiceId: "sandbox-service",
			Name:               input.ResourceName,
			MimeType:           input.MimeType,
			SizeBytes:          12,
			ContentHash:        "sha256:abc",
		},
	}, nil
}

func (f *fakeWorkspaceHostClient) ImportResourceToWorkspace(_ context.Context, input sandboxclient.ImportResourceInput, reader io.Reader) (*sandboxv1.ImportResourceToWorkspaceResponse, error) {
	f.lastImportInput = input
	if reader != nil {
		body, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		f.lastImportBody = string(body)
	}
	if f.importErr != nil {
		return nil, f.importErr
	}
	if f.importResp != nil {
		return f.importResp, nil
	}
	return &sandboxv1.ImportResourceToWorkspaceResponse{
		Path:        pathRef("sandbox-service", input.ServiceWorkspaceID, "local-process", input.DestinationPath),
		SizeBytes:   int64(len(f.lastImportBody)),
		ContentHash: input.Resource.GetContentHash(),
		MimeType:    input.Resource.GetMimeType(),
	}, nil
}

func (f *fakeWorkspaceHostClient) ExecWorkspaceCommand(_ context.Context, input sandboxclient.ExecWorkspaceCommandInput) (*sandboxv1.ExecWorkspaceCommandResponse, error) {
	f.lastExecInput = input
	if f.execErr != nil {
		return nil, f.execErr
	}
	if f.execResp != nil {
		return f.execResp, nil
	}
	return &sandboxv1.ExecWorkspaceCommandResponse{
		Workspace: &workspacev1.WorkspaceHostRef{
			ServiceId:          "sandbox-service",
			ServiceWorkspaceId: input.ServiceWorkspaceID,
			SandboxProfileId:   "local-process",
		},
		ExitCode: 0,
		Stdout:   []byte("ok"),
	}, nil
}

func (f *fakeWorkspaceHostClient) Close() error { return nil }

func sandboxDescriptor(profileIDs ...string) *capabilityv1.CapabilityDescriptor {
	profiles := make([]*capabilityv1.SandboxProfile, 0, len(profileIDs))
	for _, profileID := range profileIDs {
		profiles = append(profiles, &capabilityv1.SandboxProfile{
			Id:      profileID,
			Status:  capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_EXPERIMENTAL,
			Default: len(profiles) == 0,
		})
	}
	return &capabilityv1.CapabilityDescriptor{SandboxProfiles: profiles}
}

func testPolicies() conf.SandboxPolicies {
	return conf.SandboxPolicies{
		Global: conf.SandboxPolicyConfig{
			DefaultProfileID:  "local-process",
			AllowedProfileIDs: []string{"local-process"},
		},
		Tenants: map[string]conf.SandboxPolicyConfig{
			"tenant-a": {
				DefaultProfileID:  "cloud-vm",
				AllowedProfileIDs: []string{"cloud-vm"},
			},
		},
		Users: map[string]conf.SandboxPolicyConfig{
			"alice": {
				DefaultProfileID:  "cloud-vm",
				AllowedProfileIDs: []string{"cloud-vm"},
			},
			"bob": {
				DefaultProfileID:  "local-process",
				AllowedProfileIDs: []string{"local-process", "cloud-vm"},
			},
		},
	}
}

func TestWorkspaceServiceExecSessionWorkspaceCommand(t *testing.T) {
	workspaceStore := workspacedomain.NewMemoryStore()
	client := &fakeWorkspaceHostClient{}
	events := NewEventService("agent-control-plane", eventstore.NewMemoryStore())
	service := NewWorkspaceService(workspaceStore, client, "sandbox-service", "local-process").WithEvents(events)
	if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}

	resp, err := service.ExecSessionWorkspaceCommand(context.Background(), ExecSessionWorkspaceCommandInput{
		SessionID:      "sess-1",
		UserID:         "user-1",
		Command:        "go",
		Args:           []string{"test"},
		CWD:            "src",
		Env:            map[string]string{"GOFLAGS": "-count=1"},
		Timeout:        time.Second,
		MaxStdoutBytes: 10,
		MaxStderrBytes: 11,
	})
	if err != nil {
		t.Fatalf("ExecSessionWorkspaceCommand returned error: %v", err)
	}
	if string(resp.GetStdout()) != "ok" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if client.lastExecInput.SessionID != "sess-1" ||
		client.lastExecInput.UserID != "user-1" ||
		client.lastExecInput.ServiceWorkspaceID != "ws_sess-1" ||
		client.lastExecInput.Command != "go" ||
		client.lastExecInput.CWD != "src" ||
		client.lastExecInput.Env["GOFLAGS"] != "-count=1" ||
		client.lastExecInput.Timeout != time.Second ||
		client.lastExecInput.MaxStdoutBytes != 10 ||
		client.lastExecInput.MaxStderrBytes != 11 {
		t.Fatalf("unexpected exec input: %+v", client.lastExecInput)
	}
	eventResult, err := events.List(context.Background(), eventdomain.ListFilter{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("List events returned error: %v", err)
	}
	if !hasEventType(eventResult.Events, eventdomain.TypeWorkspaceExecCompleted) {
		t.Fatalf("expected exec completed event, got %+v", eventResult.Events)
	}
}

func TestWorkspaceServiceExecSessionWorkspaceCommandErrors(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, *workspacedomain.MemoryStore, *fakeWorkspaceHostClient)
		input ExecSessionWorkspaceCommandInput
		code  codes.Code
	}{
		{
			name:  "missing command",
			input: ExecSessionWorkspaceCommandInput{SessionID: "sess-1"},
			code:  codes.InvalidArgument,
		},
		{
			name:  "missing session",
			input: ExecSessionWorkspaceCommandInput{SessionID: "missing", Command: "go"},
			code:  codes.NotFound,
		},
		{
			name: "owner mismatch",
			setup: func(t *testing.T, store *workspacedomain.MemoryStore, client *fakeWorkspaceHostClient) {
				if _, err := NewWorkspaceService(store, client, "sandbox-service", "local-process").CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
					t.Fatalf("CreateSessionWorkspace returned error: %v", err)
				}
			},
			input: ExecSessionWorkspaceCommandInput{SessionID: "sess-1", UserID: "user-2", Command: "go"},
			code:  codes.PermissionDenied,
		},
		{
			name: "sandbox error",
			setup: func(t *testing.T, store *workspacedomain.MemoryStore, client *fakeWorkspaceHostClient) {
				if _, err := NewWorkspaceService(store, client, "sandbox-service", "local-process").CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
					t.Fatalf("CreateSessionWorkspace returned error: %v", err)
				}
				client.execErr = status.Error(codes.DeadlineExceeded, "timeout")
			},
			input: ExecSessionWorkspaceCommandInput{SessionID: "sess-1", UserID: "user-1", Command: "go"},
			code:  codes.DeadlineExceeded,
		},
		{
			name: "mismatched workspace",
			setup: func(t *testing.T, store *workspacedomain.MemoryStore, client *fakeWorkspaceHostClient) {
				if _, err := NewWorkspaceService(store, client, "sandbox-service", "local-process").CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
					t.Fatalf("CreateSessionWorkspace returned error: %v", err)
				}
				client.execResp = &sandboxv1.ExecWorkspaceCommandResponse{Workspace: &workspacev1.WorkspaceHostRef{ServiceId: "sandbox-service", ServiceWorkspaceId: "other", SandboxProfileId: "local-process"}}
			},
			input: ExecSessionWorkspaceCommandInput{SessionID: "sess-1", UserID: "user-1", Command: "go"},
			code:  codes.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := workspacedomain.NewMemoryStore()
			client := &fakeWorkspaceHostClient{}
			if tt.setup != nil {
				tt.setup(t, store, client)
			}
			service := NewWorkspaceService(store, client, "sandbox-service", "local-process")
			_, err := service.ExecSessionWorkspaceCommand(context.Background(), tt.input)
			if status.Code(err) != tt.code {
				t.Fatalf("expected %s, got %v", tt.code, err)
			}
		})
	}
}

func TestWorkspaceServiceCreateSessionWorkspace(t *testing.T) {
	client := &fakeWorkspaceHostClient{}
	events := NewEventService("agent-control-plane", eventstore.NewMemoryStore())
	service := NewWorkspaceService(workspacedomain.NewMemoryStore(), client, "sandbox-service", "local-process").WithEvents(events)
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
	eventResult, err := events.List(context.Background(), eventdomain.ListFilter{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("List events returned error: %v", err)
	}
	if !hasEventType(eventResult.Events, eventdomain.TypeWorkspaceCreated) {
		t.Fatalf("expected workspace created event, got %+v", eventResult.Events)
	}
}

func TestWorkspaceServiceCreateSessionWorkspaceValidatesProfileAvailability(t *testing.T) {
	tests := []struct {
		name       string
		client     *fakeWorkspaceHostClient
		wantCode   codes.Code
		wantCreate int
	}{
		{
			name:       "available",
			client:     &fakeWorkspaceHostClient{descriptor: sandboxDescriptor("local-process")},
			wantCreate: 1,
		},
		{
			name:     "missing profile",
			client:   &fakeWorkspaceHostClient{descriptor: sandboxDescriptor("other-profile")},
			wantCode: codes.FailedPrecondition,
		},
		{
			name:     "descriptor error",
			client:   &fakeWorkspaceHostClient{descriptorErr: status.Error(codes.Unavailable, "sandbox unavailable")},
			wantCode: codes.Unavailable,
		},
		{
			name: "declared profile ignored",
			client: &fakeWorkspaceHostClient{descriptor: &capabilityv1.CapabilityDescriptor{SandboxProfiles: []*capabilityv1.SandboxProfile{
				{Id: "local-process", Status: capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED},
			}}},
			wantCode: codes.FailedPrecondition,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewWorkspaceService(workspacedomain.NewMemoryStore(), tt.client, "sandbox-service", "local-process")
			_, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1")
			if tt.wantCode == codes.OK {
				if err != nil {
					t.Fatalf("CreateSessionWorkspace returned error: %v", err)
				}
			} else if status.Code(err) != tt.wantCode {
				t.Fatalf("expected %s, got %v", tt.wantCode, err)
			}
			if tt.client.created != tt.wantCreate {
				t.Fatalf("unexpected create count: %d", tt.client.created)
			}
		})
	}
}

func TestWorkspaceServiceCreateSessionWorkspaceProfileCache(t *testing.T) {
	client := &fakeWorkspaceHostClient{descriptor: sandboxDescriptor("local-process")}
	service := NewWorkspaceService(workspacedomain.NewMemoryStore(), client, "sandbox-service", "local-process")
	if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}
	if _, err := service.CreateSessionWorkspace(context.Background(), "sess-2", "user-1"); err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}
	if client.descriptorCalls != 1 {
		t.Fatalf("expected one descriptor call, got %d", client.descriptorCalls)
	}
}

func TestWorkspaceServiceCreateSessionWorkspaceSelectsProfileThroughPolicy(t *testing.T) {
	tests := []struct {
		name        string
		resolver    sandboxpolicydomain.Resolver
		input       CreateSessionWorkspaceInput
		descriptor  *capabilityv1.CapabilityDescriptor
		wantProfile string
		wantCode    codes.Code
	}{
		{
			name:        "user default",
			resolver:    sandboxpolicydomain.NewConfigResolver(testPolicies(), "local-process"),
			input:       CreateSessionWorkspaceInput{SessionID: "sess-1", TenantID: "tenant-a", UserID: "alice"},
			descriptor:  sandboxDescriptor("local-process", "cloud-vm"),
			wantProfile: "cloud-vm",
		},
		{
			name:        "tenant default",
			resolver:    sandboxpolicydomain.NewConfigResolver(testPolicies(), "local-process"),
			input:       CreateSessionWorkspaceInput{SessionID: "sess-1", TenantID: "tenant-a", UserID: "other-user"},
			descriptor:  sandboxDescriptor("local-process", "cloud-vm"),
			wantProfile: "cloud-vm",
		},
		{
			name:        "global default",
			resolver:    sandboxpolicydomain.NewConfigResolver(testPolicies(), "local-process"),
			input:       CreateSessionWorkspaceInput{SessionID: "sess-1", UserID: "other-user"},
			descriptor:  sandboxDescriptor("local-process", "cloud-vm"),
			wantProfile: "local-process",
		},
		{
			name:        "requested profile",
			resolver:    sandboxpolicydomain.NewConfigResolver(testPolicies(), "local-process"),
			input:       CreateSessionWorkspaceInput{SessionID: "sess-1", UserID: "bob", RequestedProfileID: "cloud-vm"},
			descriptor:  sandboxDescriptor("local-process", "cloud-vm"),
			wantProfile: "cloud-vm",
		},
		{
			name:       "requested disallowed",
			resolver:   sandboxpolicydomain.NewConfigResolver(testPolicies(), "local-process"),
			input:      CreateSessionWorkspaceInput{SessionID: "sess-1", UserID: "other-user", RequestedProfileID: "cloud-vm"},
			descriptor: sandboxDescriptor("local-process", "cloud-vm"),
			wantCode:   codes.PermissionDenied,
		},
		{
			name:       "selected unavailable",
			resolver:   sandboxpolicydomain.NewConfigResolver(testPolicies(), "local-process"),
			input:      CreateSessionWorkspaceInput{SessionID: "sess-1", TenantID: "tenant-a", UserID: "alice"},
			descriptor: sandboxDescriptor("local-process"),
			wantCode:   codes.FailedPrecondition,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeWorkspaceHostClient{descriptor: tt.descriptor}
			service := NewWorkspaceServiceWithResourcesGatewayAndPolicy(workspacedomain.NewMemoryStore(), client, nil, nil, "sandbox-service", tt.resolver)
			record, err := service.CreateSessionWorkspaceWithInput(context.Background(), tt.input)
			if tt.wantCode != codes.OK {
				if status.Code(err) != tt.wantCode {
					t.Fatalf("expected %s, got %v", tt.wantCode, err)
				}
				if client.created != 0 {
					t.Fatalf("unexpected create call")
				}
				return
			}
			if err != nil {
				t.Fatalf("CreateSessionWorkspaceWithInput returned error: %v", err)
			}
			if client.lastCreateProfileID != tt.wantProfile {
				t.Fatalf("unexpected selected profile: %q", client.lastCreateProfileID)
			}
			if record.GetCurrentHost().GetSandboxProfileId() != tt.wantProfile {
				t.Fatalf("workspace record did not store selected profile: %+v", record.GetCurrentHost())
			}
		})
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

func hasEventType(events []*eventdomain.EventRecord, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
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

func TestWorkspaceServiceExportSessionWorkspacePath(t *testing.T) {
	client := &fakeWorkspaceHostClient{}
	resourceService := NewResourceService(resourcedomain.NewMemoryStore())
	service := NewWorkspaceServiceWithResources(workspacedomain.NewMemoryStore(), client, resourceService, "sandbox-service", "local-process")
	if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "owner-1"); err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}

	record, err := service.ExportSessionWorkspacePath(context.Background(), "sess-1", "user-1", "outputs/report.txt", "report.txt", "text/plain")
	if err != nil {
		t.Fatalf("ExportSessionWorkspacePath returned error: %v", err)
	}
	if client.lastExportInput.ServiceWorkspaceID != "ws_sess-1" || client.lastExportInput.Path != "outputs/report.txt" {
		t.Fatalf("unexpected export input: %+v", client.lastExportInput)
	}
	if record.GetRef().GetId() != "res_1" || record.GetRef().GetAuthorityServiceId() != "sandbox-service" {
		t.Fatalf("unexpected resource ref: %+v", record.GetRef())
	}
	if record.GetRef().GetContentHash() != "sha256:abc" || record.GetRef().GetSizeBytes() != 12 {
		t.Fatalf("resource snapshot metadata was not propagated: %+v", record.GetRef())
	}
	if record.GetOwnerUserId() != "user-1" || record.GetSessionId() != "sess-1" {
		t.Fatalf("unexpected resource owner/session: %+v", record)
	}
	if record.GetVisibility() != resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE {
		t.Fatalf("unexpected resource visibility: %s", record.GetVisibility())
	}
	source := record.GetSource()
	if source.GetType() != "sandbox_export" ||
		source.GetSourceServiceId() != "sandbox-service" ||
		source.GetServiceWorkspaceId() != "ws_sess-1" ||
		source.GetSourcePath() != "outputs/report.txt" {
		t.Fatalf("unexpected resource source: %+v", source)
	}
}

func TestWorkspaceServiceExportSessionWorkspacePathMissingSession(t *testing.T) {
	service := NewWorkspaceServiceWithResources(workspacedomain.NewMemoryStore(), &fakeWorkspaceHostClient{}, NewResourceService(resourcedomain.NewMemoryStore()), "sandbox-service", "local-process")
	_, err := service.ExportSessionWorkspacePath(context.Background(), "missing", "user-1", "report.txt", "", "")
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestWorkspaceServiceExportSessionWorkspacePathRequiresResourceService(t *testing.T) {
	client := &fakeWorkspaceHostClient{}
	service := NewWorkspaceService(workspacedomain.NewMemoryStore(), client, "sandbox-service", "local-process")
	if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "owner-1"); err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}
	_, err := service.ExportSessionWorkspacePath(context.Background(), "sess-1", "user-1", "report.txt", "", "")
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}
}

func TestWorkspaceServiceExportSessionWorkspacePathRejectsMismatchedSourceRef(t *testing.T) {
	client := &fakeWorkspaceHostClient{
		exportResp: &sandboxv1.ExportWorkspacePathResponse{
			Source: pathRef("sandbox-service", "other-workspace", "local-process", "report.txt"),
			Resource: &resourcev1.ResourceRef{
				Id:                 "res_1",
				AuthorityServiceId: "sandbox-service",
				Name:               "report.txt",
			},
		},
	}
	service := NewWorkspaceServiceWithResources(workspacedomain.NewMemoryStore(), client, NewResourceService(resourcedomain.NewMemoryStore()), "sandbox-service", "local-process")
	if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "owner-1"); err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}
	if _, err := service.ExportSessionWorkspacePath(context.Background(), "sess-1", "user-1", "report.txt", "", ""); status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
}

func TestWorkspaceServiceExportSessionWorkspacePathRejectsAuthorityMismatch(t *testing.T) {
	client := &fakeWorkspaceHostClient{
		exportResp: &sandboxv1.ExportWorkspacePathResponse{
			Source: pathRef("sandbox-service", "ws_sess-1", "local-process", "report.txt"),
			Resource: &resourcev1.ResourceRef{
				Id:                 "res_1",
				AuthorityServiceId: "other-service",
				Name:               "report.txt",
			},
		},
	}
	service := NewWorkspaceServiceWithResources(workspacedomain.NewMemoryStore(), client, NewResourceService(resourcedomain.NewMemoryStore()), "sandbox-service", "local-process")
	if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "owner-1"); err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}
	if _, err := service.ExportSessionWorkspacePath(context.Background(), "sess-1", "user-1", "report.txt", "", ""); status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
}

func TestWorkspaceServiceExportSessionWorkspacePathPropagatesErrors(t *testing.T) {
	tests := []struct {
		name       string
		client     *fakeWorkspaceHostClient
		wantStatus codes.Code
	}{
		{
			name:       "sandbox export error",
			client:     &fakeWorkspaceHostClient{exportErr: status.Error(codes.PermissionDenied, "denied")},
			wantStatus: codes.PermissionDenied,
		},
		{
			name: "resource registration error",
			client: &fakeWorkspaceHostClient{exportResp: &sandboxv1.ExportWorkspacePathResponse{
				Source: pathRef("sandbox-service", "ws_sess-1", "local-process", "report.txt"),
				Resource: &resourcev1.ResourceRef{
					Id:                 "res_1",
					AuthorityServiceId: "sandbox-service",
				},
			}},
			wantStatus: codes.InvalidArgument,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewWorkspaceServiceWithResources(workspacedomain.NewMemoryStore(), tt.client, NewResourceService(resourcedomain.NewMemoryStore()), "sandbox-service", "local-process")
			if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "owner-1"); err != nil {
				t.Fatalf("CreateSessionWorkspace returned error: %v", err)
			}
			if _, err := service.ExportSessionWorkspacePath(context.Background(), "sess-1", "user-1", "report.txt", "", ""); status.Code(err) != tt.wantStatus {
				t.Fatalf("expected %s, got %v", tt.wantStatus, err)
			}
		})
	}
}

func TestWorkspaceServiceImportResourceToSessionWorkspace(t *testing.T) {
	workspaceStore := workspacedomain.NewMemoryStore()
	client := &fakeWorkspaceHostClient{}
	resourceStore := resourcedomain.NewMemoryStore()
	resourceRecord := registerImportResource(t, resourceStore, "res_1", "user-1", "data/report.txt")
	gateway := NewResourceGatewayService(resourceStore, map[string]ResourceAuthorityClient{
		"sandbox-service": &fakeAuthorityClient{stream: &fakeResourceContentClient{
			chunks: []*resourcev1.OpenResourceResponse{
				{Resource: resourceRecord.GetRef(), Data: []byte("hello ")},
				{Data: []byte("world")},
			},
		}},
	})
	service := NewWorkspaceServiceWithResourcesAndGateway(workspaceStore, client, NewResourceService(resourceStore), gateway, "sandbox-service", "local-process")
	if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}

	resp, err := service.ImportResourceToSessionWorkspace(context.Background(), "sess-1", "user-1", "res_1", "inputs/report.txt", sandboxv1.ImportConflictPolicy_IMPORT_CONFLICT_POLICY_OVERWRITE)
	if err != nil {
		t.Fatalf("ImportResourceToSessionWorkspace returned error: %v", err)
	}
	if client.lastImportInput.ServiceWorkspaceID != "ws_sess-1" ||
		client.lastImportInput.DestinationPath != "inputs/report.txt" ||
		client.lastImportInput.ConflictPolicy != sandboxv1.ImportConflictPolicy_IMPORT_CONFLICT_POLICY_OVERWRITE ||
		client.lastImportInput.Resource.GetId() != "res_1" {
		t.Fatalf("unexpected import input: %+v", client.lastImportInput)
	}
	if client.lastImportBody != "hello world" {
		t.Fatalf("unexpected import body: %q", client.lastImportBody)
	}
	if resp.GetPath().GetPath() != "inputs/report.txt" {
		t.Fatalf("unexpected import response: %+v", resp)
	}
}

func TestWorkspaceServiceImportResourceUsesSafeDefaultDestination(t *testing.T) {
	workspaceStore := workspacedomain.NewMemoryStore()
	client := &fakeWorkspaceHostClient{}
	resourceStore := resourcedomain.NewMemoryStore()
	resourceRecord := registerImportResource(t, resourceStore, "res_1", "user-1", "../../bad\r\nname.txt")
	gateway := NewResourceGatewayService(resourceStore, map[string]ResourceAuthorityClient{
		"sandbox-service": &fakeAuthorityClient{stream: &fakeResourceContentClient{
			chunks: []*resourcev1.OpenResourceResponse{{Resource: resourceRecord.GetRef(), Data: []byte("x")}},
		}},
	})
	service := NewWorkspaceServiceWithResourcesAndGateway(workspaceStore, client, NewResourceService(resourceStore), gateway, "sandbox-service", "local-process")
	if _, err := service.CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
		t.Fatalf("CreateSessionWorkspace returned error: %v", err)
	}

	_, err := service.ImportResourceToSessionWorkspace(context.Background(), "sess-1", "user-1", "res_1", "", sandboxv1.ImportConflictPolicy_IMPORT_CONFLICT_POLICY_FAIL_IF_EXISTS)
	if err != nil {
		t.Fatalf("ImportResourceToSessionWorkspace returned error: %v", err)
	}
	if client.lastImportInput.DestinationPath != ".._.._badname.txt" {
		t.Fatalf("unexpected default destination: %q", client.lastImportInput.DestinationPath)
	}
}

func TestWorkspaceServiceImportResourceToSessionWorkspaceErrors(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*testing.T, *workspacedomain.MemoryStore, *resourcedomain.MemoryStore, *fakeWorkspaceHostClient) *ResourceGatewayService
		sessionID string
		userID    string
		code      codes.Code
	}{
		{
			name:      "missing session",
			sessionID: "missing",
			userID:    "user-1",
			setup: func(t *testing.T, workspaceStore *workspacedomain.MemoryStore, resourceStore *resourcedomain.MemoryStore, client *fakeWorkspaceHostClient) *ResourceGatewayService {
				record := registerImportResource(t, resourceStore, "res_1", "user-1", "file.txt")
				return NewResourceGatewayService(resourceStore, map[string]ResourceAuthorityClient{"sandbox-service": &fakeAuthorityClient{stream: &fakeResourceContentClient{chunks: []*resourcev1.OpenResourceResponse{{Resource: record.GetRef()}}}}})
			},
			code: codes.NotFound,
		},
		{
			name:      "owner mismatch",
			sessionID: "sess-1",
			userID:    "user-2",
			setup: func(t *testing.T, workspaceStore *workspacedomain.MemoryStore, resourceStore *resourcedomain.MemoryStore, client *fakeWorkspaceHostClient) *ResourceGatewayService {
				if _, err := NewWorkspaceService(workspaceStore, client, "sandbox-service", "local-process").CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
					t.Fatalf("CreateSessionWorkspace returned error: %v", err)
				}
				record := registerImportResource(t, resourceStore, "res_1", "user-1", "file.txt")
				return NewResourceGatewayService(resourceStore, map[string]ResourceAuthorityClient{"sandbox-service": &fakeAuthorityClient{stream: &fakeResourceContentClient{chunks: []*resourcev1.OpenResourceResponse{{Resource: record.GetRef()}}}}})
			},
			code: codes.PermissionDenied,
		},
		{
			name:      "sandbox import error",
			sessionID: "sess-1",
			userID:    "user-1",
			setup: func(t *testing.T, workspaceStore *workspacedomain.MemoryStore, resourceStore *resourcedomain.MemoryStore, client *fakeWorkspaceHostClient) *ResourceGatewayService {
				if _, err := NewWorkspaceService(workspaceStore, client, "sandbox-service", "local-process").CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
					t.Fatalf("CreateSessionWorkspace returned error: %v", err)
				}
				client.importErr = status.Error(codes.AlreadyExists, "exists")
				record := registerImportResource(t, resourceStore, "res_1", "user-1", "file.txt")
				return NewResourceGatewayService(resourceStore, map[string]ResourceAuthorityClient{"sandbox-service": &fakeAuthorityClient{stream: &fakeResourceContentClient{chunks: []*resourcev1.OpenResourceResponse{{Resource: record.GetRef(), Data: []byte("x")}}}}})
			},
			code: codes.AlreadyExists,
		},
		{
			name:      "mismatched returned workspace",
			sessionID: "sess-1",
			userID:    "user-1",
			setup: func(t *testing.T, workspaceStore *workspacedomain.MemoryStore, resourceStore *resourcedomain.MemoryStore, client *fakeWorkspaceHostClient) *ResourceGatewayService {
				if _, err := NewWorkspaceService(workspaceStore, client, "sandbox-service", "local-process").CreateSessionWorkspace(context.Background(), "sess-1", "user-1"); err != nil {
					t.Fatalf("CreateSessionWorkspace returned error: %v", err)
				}
				client.importResp = &sandboxv1.ImportResourceToWorkspaceResponse{Path: pathRef("sandbox-service", "other", "local-process", "file.txt")}
				record := registerImportResource(t, resourceStore, "res_1", "user-1", "file.txt")
				return NewResourceGatewayService(resourceStore, map[string]ResourceAuthorityClient{"sandbox-service": &fakeAuthorityClient{stream: &fakeResourceContentClient{chunks: []*resourcev1.OpenResourceResponse{{Resource: record.GetRef(), Data: []byte("x")}}}}})
			},
			code: codes.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspaceStore := workspacedomain.NewMemoryStore()
			resourceStore := resourcedomain.NewMemoryStore()
			client := &fakeWorkspaceHostClient{}
			gateway := tt.setup(t, workspaceStore, resourceStore, client)
			service := NewWorkspaceServiceWithResourcesAndGateway(workspaceStore, client, NewResourceService(resourceStore), gateway, "sandbox-service", "local-process")
			_, err := service.ImportResourceToSessionWorkspace(context.Background(), tt.sessionID, tt.userID, "res_1", "file.txt", sandboxv1.ImportConflictPolicy_IMPORT_CONFLICT_POLICY_FAIL_IF_EXISTS)
			if status.Code(err) != tt.code {
				t.Fatalf("expected %s, got %v", tt.code, err)
			}
		})
	}
}

func registerImportResource(t *testing.T, store *resourcedomain.MemoryStore, resourceID string, owner string, name string) *resourcev1.ResourceRecord {
	t.Helper()
	record, err := store.Register(context.Background(), &resourcev1.ResourceRecord{
		Ref: &resourcev1.ResourceRef{
			Id:                 resourceID,
			AuthorityServiceId: "sandbox-service",
			Name:               name,
			MimeType:           "text/plain",
			SizeBytes:          11,
			ContentHash:        "sha256:abc",
		},
		OwnerUserId: owner,
		Status:      resourcev1.ResourceStatus_RESOURCE_STATUS_AVAILABLE,
		Visibility:  resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE,
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	return record
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
