package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	sandboxclient "github.com/lxjf12138/acorn/services/agent-control-plane/internal/client/sandbox"
	workspacedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/workspace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type WorkspaceService struct {
	mu               sync.Mutex
	store            workspacedomain.Store
	sandboxClient    sandboxclient.WorkspaceHostClient
	sandboxServiceID string
	sandboxProfileID string
}

func NewWorkspaceService(store workspacedomain.Store, sandboxClient sandboxclient.WorkspaceHostClient, sandboxServiceID string, sandboxProfileID string) *WorkspaceService {
	return &WorkspaceService{
		store:            store,
		sandboxClient:    sandboxClient,
		sandboxServiceID: sandboxServiceID,
		sandboxProfileID: sandboxProfileID,
	}
}

func (s *WorkspaceService) CreateSessionWorkspace(ctx context.Context, sessionID string, ownerUserID string) (*workspacev1.WorkspaceRecord, error) {
	if sessionID == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}
	if ownerUserID == "" {
		ownerUserID = "dev-user"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok, err := s.store.GetBySession(ctx, sessionID); err != nil {
		return nil, err
	} else if ok {
		return toProto(existing), nil
	}

	hosted, err := s.sandboxClient.CreateHostedWorkspace(ctx, sessionID, ownerUserID, s.sandboxProfileID, fmt.Sprintf("workspace for session %s", sessionID))
	if err != nil {
		return nil, err
	}
	if err := validateHostedWorkspace(hosted, s.sandboxServiceID, s.sandboxProfileID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	record, err := s.store.Create(ctx, workspacedomain.Record{
		SessionID:   sessionID,
		OwnerUserID: ownerUserID,
		Status:      workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
		CurrentHost: hosted.GetRef(),
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return nil, err
	}
	return toProto(record), nil
}

func validateHostedWorkspace(hosted *workspacev1.HostedWorkspace, expectedServiceID string, expectedProfileID string) error {
	if hosted == nil {
		return status.Error(codes.Internal, "sandbox host returned empty workspace")
	}
	ref := hosted.GetRef()
	if ref == nil {
		return status.Error(codes.Internal, "sandbox host returned workspace without ref")
	}
	if ref.GetServiceId() == "" {
		return status.Error(codes.Internal, "sandbox host returned workspace without service_id")
	}
	if ref.GetServiceWorkspaceId() == "" {
		return status.Error(codes.Internal, "sandbox host returned workspace without service_workspace_id")
	}
	if ref.GetSandboxProfileId() == "" {
		return status.Error(codes.Internal, "sandbox host returned workspace without sandbox_profile_id")
	}
	if ref.GetServiceId() != expectedServiceID {
		return status.Errorf(codes.Internal, "sandbox host returned workspace for unexpected service_id: %s", ref.GetServiceId())
	}
	if ref.GetSandboxProfileId() != expectedProfileID {
		return status.Errorf(codes.Internal, "sandbox host returned workspace for unexpected sandbox_profile_id: %s", ref.GetSandboxProfileId())
	}
	return nil
}

func (s *WorkspaceService) GetSessionWorkspace(ctx context.Context, sessionID string) (*workspacev1.WorkspaceRecord, error) {
	if sessionID == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}
	record, ok, err := s.store.GetBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, status.Error(codes.NotFound, "workspace record not found for session")
	}
	return toProto(record), nil
}

func toProto(record workspacedomain.Record) *workspacev1.WorkspaceRecord {
	return &workspacev1.WorkspaceRecord{
		Id:           record.ID,
		SessionId:    record.SessionID,
		OwnerUserId:  record.OwnerUserID,
		Status:       record.Status,
		CurrentHost:  cloneHostRef(record.CurrentHost),
		CreatedAt:    timestamppb.New(record.CreatedAt),
		UpdatedAt:    timestamppb.New(record.UpdatedAt),
		MetadataJson: append([]byte(nil), record.MetadataJSON...),
	}
}

func cloneHostRef(ref *workspacev1.WorkspaceHostRef) *workspacev1.WorkspaceHostRef {
	if ref == nil {
		return nil
	}
	return &workspacev1.WorkspaceHostRef{
		ServiceId:          ref.GetServiceId(),
		ServiceWorkspaceId: ref.GetServiceWorkspaceId(),
		SandboxProfileId:   ref.GetSandboxProfileId(),
	}
}

func IsWorkspaceRecordNotFound(err error) bool {
	return errors.Is(err, workspacedomain.ErrNotFound) || status.Code(err) == codes.NotFound
}
