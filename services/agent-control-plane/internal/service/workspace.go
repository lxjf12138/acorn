package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
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

type SessionWorkspaceState struct {
	Record *workspacev1.WorkspaceRecord    `json:"workspace"`
	State  *sandboxv1.HostedWorkspaceState `json:"state"`
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

func validateHostedWorkspace(hosted *sandboxv1.HostedWorkspace, expectedServiceID string, expectedProfileID string) error {
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

func (s *WorkspaceService) GetSessionWorkspaceState(ctx context.Context, sessionID string) (*SessionWorkspaceState, error) {
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
	if record.CurrentHost == nil || record.CurrentHost.GetServiceWorkspaceId() == "" {
		return nil, status.Error(codes.FailedPrecondition, "workspace record has no current host")
	}
	state, err := s.sandboxClient.GetHostedWorkspaceState(ctx, record.SessionID, record.OwnerUserID, record.CurrentHost.GetServiceWorkspaceId())
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, status.Error(codes.FailedPrecondition, "workspace host state unavailable")
		}
		return nil, err
	}
	if err := validateHostedWorkspaceState(state, record.CurrentHost); err != nil {
		return nil, err
	}
	return &SessionWorkspaceState{
		Record: toProto(record),
		State:  state,
	}, nil
}

func (s *WorkspaceService) ListSessionWorkspaceDir(ctx context.Context, sessionID string, userID string, path string, pageSize int32, pageToken string) (*sandboxv1.ListWorkspaceDirResponse, error) {
	record, err := s.getWorkspaceRecordForView(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	resp, err := s.sandboxClient.ListWorkspaceDir(ctx, sandboxclient.ListWorkspaceDirInput{
		SessionID:          record.SessionID,
		UserID:             userIDOrRecord(userID, record.OwnerUserID),
		ServiceWorkspaceID: record.CurrentHost.GetServiceWorkspaceId(),
		Path:               path,
		PageSize:           pageSize,
		PageToken:          pageToken,
	})
	if err != nil {
		return nil, err
	}
	if err := validateWorkspacePathRef(resp.GetDirectory(), record.CurrentHost); err != nil {
		return nil, err
	}
	for _, entry := range resp.GetEntries() {
		if err := validateWorkspacePathRef(entry.GetRef(), record.CurrentHost); err != nil {
			return nil, err
		}
	}
	return resp, nil
}

func (s *WorkspaceService) PreviewSessionWorkspaceFile(ctx context.Context, sessionID string, userID string, path string, maxBytes int64) (*sandboxv1.PreviewWorkspaceFileResponse, error) {
	record, err := s.getWorkspaceRecordForView(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	resp, err := s.sandboxClient.PreviewWorkspaceFile(ctx, sandboxclient.PreviewWorkspaceFileInput{
		SessionID:          record.SessionID,
		UserID:             userIDOrRecord(userID, record.OwnerUserID),
		ServiceWorkspaceID: record.CurrentHost.GetServiceWorkspaceId(),
		Path:               path,
		MaxBytes:           maxBytes,
	})
	if err != nil {
		return nil, err
	}
	if err := validateWorkspacePathRef(resp.GetFile(), record.CurrentHost); err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *WorkspaceService) getWorkspaceRecordForView(ctx context.Context, sessionID string) (workspacedomain.Record, error) {
	if sessionID == "" {
		return workspacedomain.Record{}, status.Error(codes.InvalidArgument, "session_id is required")
	}
	record, ok, err := s.store.GetBySession(ctx, sessionID)
	if err != nil {
		return workspacedomain.Record{}, err
	}
	if !ok {
		return workspacedomain.Record{}, status.Error(codes.NotFound, "workspace record not found for session")
	}
	if record.CurrentHost == nil || record.CurrentHost.GetServiceWorkspaceId() == "" {
		return workspacedomain.Record{}, status.Error(codes.FailedPrecondition, "workspace record has no current host")
	}
	return record, nil
}

func validateHostedWorkspaceState(state *sandboxv1.HostedWorkspaceState, expectedRef *workspacev1.WorkspaceHostRef) error {
	if state == nil {
		return status.Error(codes.Internal, "sandbox host returned empty workspace state")
	}
	ref := state.GetRef()
	if ref == nil {
		return status.Error(codes.Internal, "sandbox host returned workspace state without ref")
	}
	if expectedRef == nil {
		return status.Error(codes.FailedPrecondition, "workspace record has no current host")
	}
	if ref.GetServiceId() != expectedRef.GetServiceId() {
		return status.Errorf(codes.Internal, "sandbox host returned state for unexpected service_id: %s", ref.GetServiceId())
	}
	if ref.GetServiceWorkspaceId() != expectedRef.GetServiceWorkspaceId() {
		return status.Errorf(codes.Internal, "sandbox host returned state for unexpected service_workspace_id: %s", ref.GetServiceWorkspaceId())
	}
	if ref.GetSandboxProfileId() != expectedRef.GetSandboxProfileId() {
		return status.Errorf(codes.Internal, "sandbox host returned state for unexpected sandbox_profile_id: %s", ref.GetSandboxProfileId())
	}
	return nil
}

func validateWorkspacePathRef(ref *sandboxv1.WorkspacePathRef, expectedRef *workspacev1.WorkspaceHostRef) error {
	if ref == nil {
		return status.Error(codes.Internal, "sandbox host returned workspace path without ref")
	}
	if expectedRef == nil {
		return status.Error(codes.FailedPrecondition, "workspace record has no current host")
	}
	workspaceRef := ref.GetWorkspace()
	if workspaceRef == nil {
		return status.Error(codes.Internal, "sandbox host returned workspace path without workspace ref")
	}
	if workspaceRef.GetServiceId() != expectedRef.GetServiceId() {
		return status.Errorf(codes.Internal, "sandbox host returned path for unexpected service_id: %s", workspaceRef.GetServiceId())
	}
	if workspaceRef.GetServiceWorkspaceId() != expectedRef.GetServiceWorkspaceId() {
		return status.Errorf(codes.Internal, "sandbox host returned path for unexpected service_workspace_id: %s", workspaceRef.GetServiceWorkspaceId())
	}
	if workspaceRef.GetSandboxProfileId() != expectedRef.GetSandboxProfileId() {
		return status.Errorf(codes.Internal, "sandbox host returned path for unexpected sandbox_profile_id: %s", workspaceRef.GetSandboxProfileId())
	}
	return nil
}

func userIDOrRecord(userID string, ownerUserID string) string {
	if userID != "" {
		return userID
	}
	return ownerUserID
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
