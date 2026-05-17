package service

import (
	"context"
	"errors"
	"time"

	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/attachment"
	backenddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/backend"
	profiledomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/profile"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type workspaceAttachmentPreparer interface {
	PrepareLocalProcessAttachment(ctx context.Context, serviceWorkspaceID string, readOnly bool) (*attachment.WorkspaceAttachment, error)
}

type WorkspaceExecService struct {
	sandboxv1.UnimplementedWorkspaceExecServiceServer

	serviceID         string
	workspaceStore    workspacedomain.Store
	profiles          profiledomain.Registry
	attachmentService workspaceAttachmentPreparer
	backend           backenddomain.SandboxBackend
}

func NewWorkspaceExecService(serviceID string, workspaceStore workspacedomain.Store, profiles profiledomain.Registry, attachmentService workspaceAttachmentPreparer, backend backenddomain.SandboxBackend) *WorkspaceExecService {
	return &WorkspaceExecService{
		serviceID:         serviceID,
		workspaceStore:    workspaceStore,
		profiles:          profiles,
		attachmentService: attachmentService,
		backend:           backend,
	}
}

func (s *WorkspaceExecService) ExecWorkspaceCommand(ctx context.Context, req *sandboxv1.ExecWorkspaceCommandRequest) (*sandboxv1.ExecWorkspaceCommandResponse, error) {
	if req.GetServiceWorkspaceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "service_workspace_id is required")
	}
	if req.GetCommand() == "" {
		return nil, status.Error(codes.InvalidArgument, "command is required")
	}
	if s.attachmentService == nil || s.backend == nil {
		return nil, status.Error(codes.FailedPrecondition, "workspace exec is not configured")
	}
	workspace, err := s.workspaceStore.Get(ctx, req.GetServiceWorkspaceId())
	if errors.Is(err, workspacedomain.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "hosted workspace not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get hosted workspace: %v", err)
	}
	profile, err := s.execProfile(workspace.SandboxProfileID)
	if err != nil {
		return nil, err
	}
	att, err := s.attachmentService.PrepareLocalProcessAttachment(ctx, workspace.ID, false)
	if err != nil {
		return nil, err
	}
	lease, err := s.backend.Acquire(ctx, backenddomain.AcquireRequest{
		WorkspaceID: workspace.ID,
		Attachment:  att,
		ProfileID:   profile.ID,
	})
	if err != nil {
		return nil, mapBackendError(err)
	}
	defer func() {
		_ = s.backend.Release(ctx, lease)
	}()
	timeout := time.Duration(0)
	if req.GetTimeout() != nil {
		timeout = req.GetTimeout().AsDuration()
	}
	result, err := s.backend.Exec(ctx, lease, backenddomain.ExecRequest{
		Command:        req.GetCommand(),
		Args:           append([]string(nil), req.GetArgs()...),
		CWD:            req.GetCwd(),
		Env:            cloneStringMap(req.GetEnv()),
		Timeout:        timeout,
		MaxStdoutBytes: req.GetMaxStdoutBytes(),
		MaxStderrBytes: req.GetMaxStderrBytes(),
	})
	if err != nil {
		return nil, mapBackendError(err)
	}
	return &sandboxv1.ExecWorkspaceCommandResponse{
		Workspace: &workspacev1.WorkspaceHostRef{
			ServiceId:          s.serviceID,
			ServiceWorkspaceId: workspace.ID,
			SandboxProfileId:   workspace.SandboxProfileID,
		},
		ExitCode:        int32(result.ExitCode),
		Stdout:          append([]byte(nil), result.Stdout...),
		Stderr:          append([]byte(nil), result.Stderr...),
		StdoutTruncated: result.StdoutTruncated,
		StderrTruncated: result.StderrTruncated,
		ErrorMessage:    result.ErrorMessage,
	}, nil
}

func (s *WorkspaceExecService) execProfile(profileID string) (*profiledomain.Profile, error) {
	if s.profiles == nil {
		return nil, status.Error(codes.FailedPrecondition, "sandbox profile registry is unavailable")
	}
	profile, err := s.profiles.Get(profileID)
	if errors.Is(err, profiledomain.ErrProfileNotFound) {
		return nil, status.Error(codes.FailedPrecondition, "sandbox profile is unavailable")
	}
	if errors.Is(err, profiledomain.ErrProfileDisabled) {
		return nil, status.Error(codes.FailedPrecondition, "sandbox profile is disabled")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve sandbox profile: %v", err)
	}
	if !profile.HasCapability(profiledomain.CapabilityWorkspaceExec) {
		return nil, status.Error(codes.FailedPrecondition, "sandbox profile does not support workspace exec")
	}
	if s.backend != nil && profile.BackendID != "" && profile.BackendID != s.backend.ID() {
		return nil, status.Error(codes.FailedPrecondition, "sandbox profile is not backed by configured backend")
	}
	return profile, nil
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
