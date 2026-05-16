package service

import (
	"context"
	"errors"
	"time"

	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/descriptor"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type WorkspaceService struct {
	sandboxv1.UnimplementedWorkspaceHostServiceServer

	serviceID string
	profiles  *descriptor.Source
	store     workspacedomain.Store
}

func NewWorkspaceService(serviceID string, profiles *descriptor.Source, store workspacedomain.Store) *WorkspaceService {
	return &WorkspaceService{
		serviceID: serviceID,
		profiles:  profiles,
		store:     store,
	}
}

func (s *WorkspaceService) CreateHostedWorkspace(ctx context.Context, req *sandboxv1.CreateHostedWorkspaceRequest) (*sandboxv1.CreateHostedWorkspaceResponse, error) {
	profile, err := s.resolveProfile(req.GetSandboxProfileId())
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	workspace, err := s.store.Create(ctx, workspacedomain.Workspace{
		SandboxProfileID: profile.GetId(),
		DisplayName:      req.GetDisplayName(),
		Status:           workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
		CreatedAt:        now,
		UpdatedAt:        now,
		MetadataJSON:     req.GetMetadataJson(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create hosted workspace: %v", err)
	}
	return &sandboxv1.CreateHostedWorkspaceResponse{
		Workspace: s.toProto(workspace),
	}, nil
}

func (s *WorkspaceService) GetHostedWorkspace(ctx context.Context, req *sandboxv1.GetHostedWorkspaceRequest) (*sandboxv1.GetHostedWorkspaceResponse, error) {
	if req.GetServiceWorkspaceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "service_workspace_id is required")
	}
	workspace, err := s.store.Get(ctx, req.GetServiceWorkspaceId())
	if errors.Is(err, workspacedomain.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "hosted workspace not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get hosted workspace: %v", err)
	}
	return &sandboxv1.GetHostedWorkspaceResponse{
		Workspace: s.toProto(workspace),
	}, nil
}

func (s *WorkspaceService) GetHostedWorkspaceState(ctx context.Context, req *sandboxv1.GetHostedWorkspaceStateRequest) (*sandboxv1.GetHostedWorkspaceStateResponse, error) {
	if req.GetServiceWorkspaceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "service_workspace_id is required")
	}
	workspace, err := s.store.Get(ctx, req.GetServiceWorkspaceId())
	if errors.Is(err, workspacedomain.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "hosted workspace not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get hosted workspace state: %v", err)
	}
	return &sandboxv1.GetHostedWorkspaceStateResponse{
		State: s.toStateProto(workspace),
	}, nil
}

func (s *WorkspaceService) resolveProfile(profileID string) (*capabilityv1.SandboxProfile, error) {
	if profileID == "" {
		profile, ok := s.profiles.DefaultSandboxProfile()
		if !ok {
			return nil, status.Error(codes.FailedPrecondition, "default sandbox profile is unavailable")
		}
		return profile, nil
	}
	profile, ok := s.profiles.SandboxProfile(profileID)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "unknown sandbox profile: %s", profileID)
	}
	if profile.GetStatus() == capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DISABLED {
		return nil, status.Errorf(codes.FailedPrecondition, "sandbox profile is disabled: %s", profileID)
	}
	return profile, nil
}

func (s *WorkspaceService) toStateProto(workspace workspacedomain.Workspace) *sandboxv1.HostedWorkspaceState {
	return &sandboxv1.HostedWorkspaceState{
		Ref: &workspacev1.WorkspaceHostRef{
			ServiceId:          s.serviceID,
			ServiceWorkspaceId: workspace.ID,
			SandboxProfileId:   workspace.SandboxProfileID,
		},
		Status:  workspace.Status,
		Summary: "empty workspace",
		Facts: []*sandboxv1.WorkspaceStateFact{
			{
				Key:   "profile",
				Value: workspace.SandboxProfileID,
			},
			{
				Key:   "workspace_status",
				Value: workspace.Status.String(),
			},
		},
		GeneratedAt: timestamppb.New(time.Now().UTC()),
	}
}

func (s *WorkspaceService) toProto(workspace workspacedomain.Workspace) *sandboxv1.HostedWorkspace {
	return &sandboxv1.HostedWorkspace{
		Ref: &workspacev1.WorkspaceHostRef{
			ServiceId:          s.serviceID,
			ServiceWorkspaceId: workspace.ID,
			SandboxProfileId:   workspace.SandboxProfileID,
		},
		Status:       workspace.Status,
		DisplayName:  workspace.DisplayName,
		CreatedAt:    timestamppb.New(workspace.CreatedAt),
		UpdatedAt:    timestamppb.New(workspace.UpdatedAt),
		MetadataJson: append([]byte(nil), workspace.MetadataJSON...),
	}
}
