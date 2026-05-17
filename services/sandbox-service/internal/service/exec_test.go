package service

import (
	"context"
	"errors"
	"testing"
	"time"

	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/attachment"
	backenddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/backend"
	profiledomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/profile"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestWorkspaceExecServiceExecWorkspaceCommand(t *testing.T) {
	store := workspacedomain.NewMemoryStore()
	workspace, err := store.Create(context.Background(), workspacedomain.Workspace{
		ID:               "ws-service",
		SandboxProfileID: "local-process-dev",
		StoreWorkspaceID: "ws-backing",
		Status:           workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
	})
	if err != nil {
		t.Fatalf("Create workspace returned error: %v", err)
	}
	attachmentSvc := &fakeAttachmentPreparer{
		att: &attachment.WorkspaceAttachment{ID: "att_ws", WorkspaceID: "ws-backing", Kind: attachment.KindLocalPath, LocalPath: "/tmp/ws"},
	}
	backend := &fakeSandboxBackend{
		result: &backenddomain.ExecResult{
			ExitCode:        3,
			Stdout:          []byte("out"),
			Stderr:          []byte("err"),
			StdoutTruncated: true,
			ErrorMessage:    "exit status 3",
		},
	}
	service := NewWorkspaceExecService("sandbox-service", store, testExecProfileRegistry("fake-backend", true), attachmentSvc, backend)

	resp, err := service.ExecWorkspaceCommand(context.Background(), &sandboxv1.ExecWorkspaceCommandRequest{
		ServiceWorkspaceId: workspace.ID,
		Command:            "go",
		Args:               []string{"test"},
		Cwd:                "src",
		Env:                map[string]string{"GOFLAGS": "-count=1"},
		Timeout:            durationpb.New(time.Second),
		MaxStdoutBytes:     10,
		MaxStderrBytes:     11,
	})
	if err != nil {
		t.Fatalf("ExecWorkspaceCommand returned error: %v", err)
	}
	if attachmentSvc.workspaceID != workspace.ID || attachmentSvc.readOnly {
		t.Fatalf("unexpected attachment request: workspace=%q readonly=%v", attachmentSvc.workspaceID, attachmentSvc.readOnly)
	}
	if backend.acquireReq.WorkspaceID != workspace.ID || backend.acquireReq.Attachment != attachmentSvc.att || backend.acquireReq.ProfileID != "local-process-dev" {
		t.Fatalf("unexpected acquire request: %+v", backend.acquireReq)
	}
	if !backend.released {
		t.Fatal("expected lease release")
	}
	if backend.execReq.Command != "go" ||
		backend.execReq.CWD != "src" ||
		backend.execReq.Env["GOFLAGS"] != "-count=1" ||
		backend.execReq.Timeout != time.Second ||
		backend.execReq.MaxStdoutBytes != 10 ||
		backend.execReq.MaxStderrBytes != 11 {
		t.Fatalf("unexpected exec request: %+v", backend.execReq)
	}
	if resp.GetWorkspace().GetServiceId() != "sandbox-service" ||
		resp.GetWorkspace().GetServiceWorkspaceId() != workspace.ID ||
		resp.GetWorkspace().GetSandboxProfileId() != "local-process-dev" ||
		resp.GetExitCode() != 3 ||
		string(resp.GetStdout()) != "out" ||
		string(resp.GetStderr()) != "err" ||
		!resp.GetStdoutTruncated() ||
		resp.GetErrorMessage() != "exit status 3" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestWorkspaceExecServiceErrors(t *testing.T) {
	store := workspacedomain.NewMemoryStore()
	if _, err := store.Create(context.Background(), workspacedomain.Workspace{ID: "ws-service", SandboxProfileID: "local-process-dev"}); err != nil {
		t.Fatalf("Create workspace returned error: %v", err)
	}
	tests := []struct {
		name     string
		req      *sandboxv1.ExecWorkspaceCommandRequest
		backend  *fakeSandboxBackend
		profiles profiledomain.Registry
		code     codes.Code
	}{
		{name: "missing workspace id", req: &sandboxv1.ExecWorkspaceCommandRequest{Command: "go"}, backend: &fakeSandboxBackend{}, code: codes.InvalidArgument},
		{name: "missing command", req: &sandboxv1.ExecWorkspaceCommandRequest{ServiceWorkspaceId: "ws-service"}, backend: &fakeSandboxBackend{}, code: codes.InvalidArgument},
		{name: "missing workspace", req: &sandboxv1.ExecWorkspaceCommandRequest{ServiceWorkspaceId: "missing", Command: "go"}, backend: &fakeSandboxBackend{}, code: codes.NotFound},
		{name: "backend timeout", req: &sandboxv1.ExecWorkspaceCommandRequest{ServiceWorkspaceId: "ws-service", Command: "go"}, backend: &fakeSandboxBackend{execErr: backenddomain.ErrExecTimeout}, profiles: testExecProfileRegistry("fake-backend", true), code: codes.DeadlineExceeded},
		{name: "start failure", req: &sandboxv1.ExecWorkspaceCommandRequest{ServiceWorkspaceId: "ws-service", Command: "go"}, backend: &fakeSandboxBackend{execErr: backenddomain.ErrExecStart}, profiles: testExecProfileRegistry("fake-backend", true), code: codes.FailedPrecondition},
		{name: "invalid cwd", req: &sandboxv1.ExecWorkspaceCommandRequest{ServiceWorkspaceId: "ws-service", Command: "go"}, backend: &fakeSandboxBackend{execErr: backenddomain.ErrInvalidCWD}, profiles: testExecProfileRegistry("fake-backend", true), code: codes.InvalidArgument},
		{name: "unknown backend", req: &sandboxv1.ExecWorkspaceCommandRequest{ServiceWorkspaceId: "ws-service", Command: "go"}, backend: &fakeSandboxBackend{execErr: errors.New("boom")}, profiles: testExecProfileRegistry("fake-backend", true), code: codes.Internal},
		{name: "profile without exec capability", req: &sandboxv1.ExecWorkspaceCommandRequest{ServiceWorkspaceId: "ws-service", Command: "go"}, backend: &fakeSandboxBackend{}, profiles: testExecProfileRegistry("fake-backend", false), code: codes.FailedPrecondition},
		{name: "profile backed by different backend", req: &sandboxv1.ExecWorkspaceCommandRequest{ServiceWorkspaceId: "ws-service", Command: "go"}, backend: &fakeSandboxBackend{}, profiles: testExecProfileRegistry("other-backend", true), code: codes.FailedPrecondition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profiles := tt.profiles
			if profiles == nil {
				profiles = testExecProfileRegistry("fake-backend", true)
			}
			service := NewWorkspaceExecService("sandbox-service", store, profiles, &fakeAttachmentPreparer{att: &attachment.WorkspaceAttachment{Kind: attachment.KindLocalPath}}, tt.backend)
			_, err := service.ExecWorkspaceCommand(context.Background(), tt.req)
			if status.Code(err) != tt.code {
				t.Fatalf("expected %s, got %v", tt.code, err)
			}
		})
	}
}

func testExecProfileRegistry(backendID string, exec bool) profiledomain.Registry {
	capabilities := []profiledomain.Capability{profiledomain.CapabilityWorkspaceView}
	if exec {
		capabilities = append(capabilities, profiledomain.CapabilityWorkspaceExec)
	}
	return profiledomain.NewMemoryRegistry([]*profiledomain.Profile{
		{
			ID:             profiledomain.LocalProcessDevID,
			Enabled:        true,
			Default:        true,
			IsolationClass: profiledomain.IsolationDevProcess,
			BackendID:      backendID,
			Capabilities:   capabilities,
		},
	})
}

type fakeAttachmentPreparer struct {
	att         *attachment.WorkspaceAttachment
	err         error
	workspaceID string
	readOnly    bool
}

func (f *fakeAttachmentPreparer) PrepareLocalProcessAttachment(_ context.Context, serviceWorkspaceID string, readOnly bool) (*attachment.WorkspaceAttachment, error) {
	f.workspaceID = serviceWorkspaceID
	f.readOnly = readOnly
	if f.err != nil {
		return nil, f.err
	}
	return f.att, nil
}

type fakeSandboxBackend struct {
	result     *backenddomain.ExecResult
	acquireErr error
	execErr    error
	releaseErr error

	acquireReq backenddomain.AcquireRequest
	execReq    backenddomain.ExecRequest
	released   bool
}

func (f *fakeSandboxBackend) ID() string   { return "fake-backend" }
func (f *fakeSandboxBackend) Kind() string { return "fake" }

func (f *fakeSandboxBackend) Acquire(_ context.Context, req backenddomain.AcquireRequest) (*backenddomain.SandboxLease, error) {
	f.acquireReq = req
	if f.acquireErr != nil {
		return nil, f.acquireErr
	}
	return &backenddomain.SandboxLease{ID: "lease_1", WorkspaceID: req.WorkspaceID, Attachment: req.Attachment}, nil
}

func (f *fakeSandboxBackend) Release(context.Context, *backenddomain.SandboxLease) error {
	f.released = true
	return f.releaseErr
}

func (f *fakeSandboxBackend) Exec(_ context.Context, _ *backenddomain.SandboxLease, req backenddomain.ExecRequest) (*backenddomain.ExecResult, error) {
	f.execReq = req
	if f.execErr != nil {
		return nil, f.execErr
	}
	if f.result != nil {
		return f.result, nil
	}
	return &backenddomain.ExecResult{}, nil
}
