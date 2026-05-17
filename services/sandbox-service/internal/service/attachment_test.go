package service

import (
	"context"
	"errors"
	"testing"
	"time"

	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/attachment"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWorkspaceAttachmentServicePrepareLocalProcessAttachment(t *testing.T) {
	store := workspacedomain.NewMemoryStore()
	workspace, err := store.Create(context.Background(), workspacedomain.Workspace{
		ID:               "ws-service",
		SandboxProfileID: "local-process",
		Status:           workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
		StoreKind:        "fake",
		StoreWorkspaceID: "ws-backing",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Create workspace returned error: %v", err)
	}
	mounter := &fakeWorkspaceMounter{
		resp: &attachment.WorkspaceAttachment{
			ID:          "att_ws",
			WorkspaceID: "ws-backing",
			Kind:        attachment.KindLocalPath,
			LocalPath:   "/tmp/workspace",
		},
	}
	service := NewWorkspaceAttachmentService(store, mounter)

	att, err := service.PrepareLocalProcessAttachment(context.Background(), workspace.ID, true)
	if err != nil {
		t.Fatalf("PrepareLocalProcessAttachment returned error: %v", err)
	}
	if att != mounter.resp {
		t.Fatalf("unexpected attachment: %+v", att)
	}
	if mounter.lastReq.WorkspaceID != "ws-backing" ||
		mounter.lastReq.Target.Kind != attachment.TargetLocalProcess ||
		!mounter.lastReq.ReadOnly {
		t.Fatalf("unexpected prepare request: %+v", mounter.lastReq)
	}
}

func TestWorkspaceAttachmentServicePrepareErrors(t *testing.T) {
	store := workspacedomain.NewMemoryStore()
	if _, err := store.Create(context.Background(), workspacedomain.Workspace{ID: "ws-service", StoreWorkspaceID: "ws-backing"}); err != nil {
		t.Fatalf("Create workspace returned error: %v", err)
	}

	tests := []struct {
		name      string
		workspace string
		mounter   attachment.WorkspaceMounter
		code      codes.Code
	}{
		{name: "missing id", workspace: "", mounter: &fakeWorkspaceMounter{}, code: codes.InvalidArgument},
		{name: "missing workspace", workspace: "missing", mounter: &fakeWorkspaceMounter{}, code: codes.NotFound},
		{name: "missing mounter", workspace: "ws-service", mounter: nil, code: codes.FailedPrecondition},
		{name: "unsupported target", workspace: "ws-service", mounter: &fakeWorkspaceMounter{err: attachment.ErrUnsupportedTarget}, code: codes.FailedPrecondition},
		{name: "attachment not ready", workspace: "ws-service", mounter: &fakeWorkspaceMounter{err: attachment.ErrAttachmentNotReady}, code: codes.FailedPrecondition},
		{name: "source missing", workspace: "ws-service", mounter: &fakeWorkspaceMounter{err: attachment.ErrWorkspaceNotFound}, code: codes.NotFound},
		{name: "unknown mounter error", workspace: "ws-service", mounter: &fakeWorkspaceMounter{err: errors.New("boom")}, code: codes.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewWorkspaceAttachmentService(store, tt.mounter)
			_, err := service.PrepareLocalProcessAttachment(context.Background(), tt.workspace, false)
			if status.Code(err) != tt.code {
				t.Fatalf("expected %s, got %v", tt.code, err)
			}
		})
	}
}

type fakeWorkspaceMounter struct {
	resp    *attachment.WorkspaceAttachment
	err     error
	lastReq attachment.PrepareRequest
}

func (f *fakeWorkspaceMounter) Kind() string { return "fake" }

func (f *fakeWorkspaceMounter) Prepare(_ context.Context, req attachment.PrepareRequest) (*attachment.WorkspaceAttachment, error) {
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	if f.resp != nil {
		return f.resp, nil
	}
	return &attachment.WorkspaceAttachment{ID: "att_default", WorkspaceID: req.WorkspaceID, Kind: attachment.KindLocalPath}, nil
}

func (f *fakeWorkspaceMounter) Release(context.Context, *attachment.WorkspaceAttachment) error {
	return nil
}
