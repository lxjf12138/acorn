package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
	commonv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/common/v1"
	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	"github.com/lxjf12138/acorn/packages/servicekit/httpx"
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
	resourceService  *ResourceService
	resourceGateway  *ResourceGatewayService
	sandboxServiceID string
	sandboxProfileID string
	profileCache     SandboxProfileCache
}

const sandboxProfileCacheTTL = 30 * time.Second

type SessionWorkspaceState struct {
	Record *workspacev1.WorkspaceRecord    `json:"workspace"`
	State  *sandboxv1.HostedWorkspaceState `json:"state"`
}

type ExecSessionWorkspaceCommandInput struct {
	SessionID string
	UserID    string

	Command string
	Args    []string
	CWD     string
	Env     map[string]string

	Timeout        time.Duration
	MaxStdoutBytes int64
	MaxStderrBytes int64
}

func NewWorkspaceService(store workspacedomain.Store, sandboxClient sandboxclient.WorkspaceHostClient, sandboxServiceID string, sandboxProfileID string) *WorkspaceService {
	return NewWorkspaceServiceWithResources(store, sandboxClient, nil, sandboxServiceID, sandboxProfileID)
}

func NewWorkspaceServiceWithResources(store workspacedomain.Store, sandboxClient sandboxclient.WorkspaceHostClient, resourceService *ResourceService, sandboxServiceID string, sandboxProfileID string) *WorkspaceService {
	return NewWorkspaceServiceWithResourcesAndGateway(store, sandboxClient, resourceService, nil, sandboxServiceID, sandboxProfileID)
}

func NewWorkspaceServiceWithResourcesAndGateway(store workspacedomain.Store, sandboxClient sandboxclient.WorkspaceHostClient, resourceService *ResourceService, resourceGateway *ResourceGatewayService, sandboxServiceID string, sandboxProfileID string) *WorkspaceService {
	return &WorkspaceService{
		store:            store,
		sandboxClient:    sandboxClient,
		resourceService:  resourceService,
		resourceGateway:  resourceGateway,
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

	if err := s.ensureSandboxProfileAvailable(ctx, s.sandboxProfileID); err != nil {
		return nil, err
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

type SandboxProfileCache struct {
	mu        sync.RWMutex
	expiresAt time.Time
	profiles  map[string]struct{}
}

func (s *WorkspaceService) ensureSandboxProfileAvailable(ctx context.Context, profileID string) error {
	if profileID == "" {
		return status.Error(codes.FailedPrecondition, "sandbox default profile is not configured")
	}
	if s.profileCache.Has(profileID, time.Now()) {
		return nil
	}
	descriptor, err := s.sandboxClient.GetCapabilityDescriptor(ctx)
	if err != nil {
		return err
	}
	profiles := availableSandboxProfiles(descriptor)
	s.profileCache.Store(profiles, time.Now().Add(sandboxProfileCacheTTL))
	if _, ok := profiles[profileID]; !ok {
		return status.Errorf(codes.FailedPrecondition, "sandbox profile unavailable: %s", profileID)
	}
	return nil
}

func (c *SandboxProfileCache) Has(profileID string, now time.Time) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.profiles == nil || now.After(c.expiresAt) {
		return false
	}
	_, ok := c.profiles[profileID]
	return ok
}

func (c *SandboxProfileCache) Store(profiles map[string]struct{}, expiresAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.profiles = make(map[string]struct{}, len(profiles))
	for profileID := range profiles {
		c.profiles[profileID] = struct{}{}
	}
	c.expiresAt = expiresAt
}

func availableSandboxProfiles(descriptor *capabilityv1.CapabilityDescriptor) map[string]struct{} {
	out := map[string]struct{}{}
	for _, profile := range descriptor.GetSandboxProfiles() {
		if profile.GetId() == "" {
			continue
		}
		if profile.GetStatus() == capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DISABLED ||
			profile.GetStatus() == capabilityv1.ImplementationStatus_IMPLEMENTATION_STATUS_DECLARED {
			continue
		}
		out[profile.GetId()] = struct{}{}
	}
	return out
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

func (s *WorkspaceService) ExportSessionWorkspacePath(ctx context.Context, sessionID string, userID string, path string, resourceName string, mimeType string) (*resourcev1.ResourceRecord, error) {
	if s.resourceService == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource service is not configured")
	}
	record, err := s.getWorkspaceRecordForView(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	ownerUserID := userIDOrRecord(userID, record.OwnerUserID)
	resp, err := s.sandboxClient.ExportWorkspacePath(ctx, sandboxclient.ExportWorkspacePathInput{
		SessionID:          record.SessionID,
		UserID:             ownerUserID,
		ServiceWorkspaceID: record.CurrentHost.GetServiceWorkspaceId(),
		Path:               path,
		ResourceName:       resourceName,
		MimeType:           mimeType,
	})
	if err != nil {
		return nil, err
	}
	if err := validateWorkspacePathRef(resp.GetSource(), record.CurrentHost); err != nil {
		return nil, err
	}
	resourceRef := resp.GetResource()
	if resourceRef == nil {
		return nil, status.Error(codes.Internal, "sandbox export returned empty resource ref")
	}
	if resourceRef.GetAuthorityServiceId() != record.CurrentHost.GetServiceId() {
		return nil, status.Errorf(codes.Internal, "sandbox export returned resource for unexpected authority_service_id: %s", resourceRef.GetAuthorityServiceId())
	}
	return s.resourceService.RegisterRecord(ctx, &resourcev1.RegisterResourceRequest{
		Scope: &commonv1.Scope{
			SessionId: record.SessionID,
			UserId:    ownerUserID,
			ServiceId: record.CurrentHost.GetServiceId(),
		},
		Ref:         resourceRef,
		OwnerUserId: ownerUserID,
		SessionId:   record.SessionID,
		Source: &resourcev1.ResourceSource{
			Type:               "sandbox_export",
			SourceServiceId:    record.CurrentHost.GetServiceId(),
			WorkspaceRecordId:  record.ID,
			ServiceWorkspaceId: record.CurrentHost.GetServiceWorkspaceId(),
			SourcePath:         resp.GetSource().GetPath(),
		},
		Visibility: resourcev1.ResourceVisibility_RESOURCE_VISIBILITY_USER_VISIBLE,
	})
}

func (s *WorkspaceService) ImportResourceToSessionWorkspace(ctx context.Context, sessionID string, userID string, resourceID string, destinationPath string, conflictPolicy sandboxv1.ImportConflictPolicy) (*sandboxv1.ImportResourceToWorkspaceResponse, error) {
	if s.resourceGateway == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource gateway is not configured")
	}
	record, err := s.getWorkspaceRecordForView(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	ownerUserID := userIDOrRecord(userID, record.OwnerUserID)
	resourceStream, err := s.resourceGateway.OpenResourceForTransfer(ctx, resourceID, ownerUserID)
	if err != nil {
		return nil, err
	}
	resourceRecord := resourceStream.Record()
	ref := resourceRecord.GetRef()
	if destinationPath == "" {
		destinationPath = httpx.SafeFilename(ref.GetName(), ref.GetId())
	}
	resp, err := s.sandboxClient.ImportResourceToWorkspace(ctx, sandboxclient.ImportResourceInput{
		SessionID:          record.SessionID,
		UserID:             ownerUserID,
		ServiceWorkspaceID: record.CurrentHost.GetServiceWorkspaceId(),
		Resource:           ref,
		DestinationPath:    destinationPath,
		ConflictPolicy:     conflictPolicy,
	}, resourceStream)
	if err != nil {
		return nil, err
	}
	if err := validateWorkspacePathRef(resp.GetPath(), record.CurrentHost); err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *WorkspaceService) ExecSessionWorkspaceCommand(ctx context.Context, input ExecSessionWorkspaceCommandInput) (*sandboxv1.ExecWorkspaceCommandResponse, error) {
	if input.Command == "" {
		return nil, status.Error(codes.InvalidArgument, "command is required")
	}
	record, err := s.getWorkspaceRecordForView(ctx, input.SessionID)
	if err != nil {
		return nil, err
	}
	userID := userIDOrRecord(input.UserID, record.OwnerUserID)
	if record.OwnerUserID != "" && userID != "" && record.OwnerUserID != userID {
		return nil, status.Error(codes.PermissionDenied, "workspace owner mismatch")
	}
	resp, err := s.sandboxClient.ExecWorkspaceCommand(ctx, sandboxclient.ExecWorkspaceCommandInput{
		SessionID:          record.SessionID,
		UserID:             userID,
		ServiceWorkspaceID: record.CurrentHost.GetServiceWorkspaceId(),
		Command:            input.Command,
		Args:               append([]string(nil), input.Args...),
		CWD:                input.CWD,
		Env:                cloneStringMap(input.Env),
		Timeout:            input.Timeout,
		MaxStdoutBytes:     input.MaxStdoutBytes,
		MaxStderrBytes:     input.MaxStderrBytes,
	})
	if err != nil {
		return nil, err
	}
	if err := validateWorkspaceHostRef(resp.GetWorkspace(), record.CurrentHost); err != nil {
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
	return validateWorkspaceHostRef(ref, expectedRef)
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
	return validateWorkspaceHostRef(workspaceRef, expectedRef)
}

func validateWorkspaceHostRef(ref *workspacev1.WorkspaceHostRef, expectedRef *workspacev1.WorkspaceHostRef) error {
	if ref == nil {
		return status.Error(codes.Internal, "sandbox host returned empty workspace ref")
	}
	if expectedRef == nil {
		return status.Error(codes.FailedPrecondition, "workspace record has no current host")
	}
	if ref.GetServiceId() != expectedRef.GetServiceId() {
		return status.Errorf(codes.Internal, "sandbox host returned workspace ref for unexpected service_id: %s", ref.GetServiceId())
	}
	if ref.GetServiceWorkspaceId() != expectedRef.GetServiceWorkspaceId() {
		return status.Errorf(codes.Internal, "sandbox host returned workspace ref for unexpected service_workspace_id: %s", ref.GetServiceWorkspaceId())
	}
	if ref.GetSandboxProfileId() != expectedRef.GetSandboxProfileId() {
		return status.Errorf(codes.Internal, "sandbox host returned workspace ref for unexpected sandbox_profile_id: %s", ref.GetSandboxProfileId())
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

func IsWorkspaceRecordNotFound(err error) bool {
	return errors.Is(err, workspacedomain.ErrNotFound) || status.Code(err) == codes.NotFound
}
