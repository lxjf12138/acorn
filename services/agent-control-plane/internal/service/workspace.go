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
	"github.com/lxjf12138/acorn/packages/core/telemetry"
	"github.com/lxjf12138/acorn/packages/servicekit/httpx"
	sandboxclient "github.com/lxjf12138/acorn/services/agent-control-plane/internal/client/sandbox"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/conf"
	eventdomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/event"
	sandboxpolicydomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/sandboxpolicy"
	workspacedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/workspace"
	"go.opentelemetry.io/otel/attribute"
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
	profileResolver  sandboxpolicydomain.Resolver
	profileCatalog   *SandboxProfileCatalog
	events           EventAppender
}

const sandboxProfileCacheTTL = 30 * time.Second

type SessionWorkspaceState struct {
	Record *workspacev1.WorkspaceRecord    `json:"workspace"`
	State  *sandboxv1.HostedWorkspaceState `json:"state"`
}

func (s *WorkspaceService) WithEvents(events EventAppender) *WorkspaceService {
	s.events = events
	return s
}

type CreateSessionWorkspaceInput struct {
	SessionID string
	TenantID  string
	UserID    string

	RequestedProfileID string
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
	return NewWorkspaceServiceWithResourcesGatewayAndPolicy(store, sandboxClient, resourceService, resourceGateway, sandboxServiceID, sandboxpolicydomain.NewConfigResolver(confSandboxPolicies(sandboxProfileID), sandboxProfileID))
}

func NewWorkspaceServiceWithResourcesGatewayAndPolicy(store workspacedomain.Store, sandboxClient sandboxclient.WorkspaceHostClient, resourceService *ResourceService, resourceGateway *ResourceGatewayService, sandboxServiceID string, profileResolver sandboxpolicydomain.Resolver) *WorkspaceService {
	return &WorkspaceService{
		store:            store,
		sandboxClient:    sandboxClient,
		resourceService:  resourceService,
		resourceGateway:  resourceGateway,
		sandboxServiceID: sandboxServiceID,
		profileResolver:  profileResolver,
		profileCatalog:   NewSandboxProfileCatalog(sandboxClient, sandboxProfileCacheTTL),
	}
}

func confSandboxPolicies(defaultProfileID string) conf.SandboxPolicies {
	return conf.SandboxPolicies{
		Global: conf.SandboxPolicyConfig{
			DefaultProfileID:  defaultProfileID,
			AllowedProfileIDs: []string{defaultProfileID},
		},
	}
}

func (s *WorkspaceService) CreateSessionWorkspace(ctx context.Context, sessionID string, ownerUserID string) (*workspacev1.WorkspaceRecord, error) {
	return s.CreateSessionWorkspaceWithInput(ctx, CreateSessionWorkspaceInput{
		SessionID: sessionID,
		UserID:    ownerUserID,
	})
}

func (s *WorkspaceService) CreateSessionWorkspaceWithInput(ctx context.Context, input CreateSessionWorkspaceInput) (*workspacev1.WorkspaceRecord, error) {
	if input.SessionID == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}
	if input.UserID == "" {
		input.UserID = "dev-user"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok, err := s.store.GetBySession(ctx, input.SessionID); err != nil {
		return nil, err
	} else if ok {
		return toProto(existing), nil
	}

	profile, err := s.selectSandboxProfile(ctx, input)
	if err != nil {
		return nil, err
	}
	hosted, err := s.sandboxClient.CreateHostedWorkspace(ctx, input.SessionID, input.UserID, profile.ProfileID, fmt.Sprintf("workspace for session %s", input.SessionID))
	if err != nil {
		return nil, err
	}
	if err := validateHostedWorkspace(hosted, s.sandboxServiceID, profile.ProfileID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	record, err := s.store.Create(ctx, workspacedomain.Record{
		SessionID:   input.SessionID,
		OwnerUserID: input.UserID,
		Status:      workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
		CurrentHost: hosted.GetRef(),
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return nil, err
	}
	protoRecord := toProto(record)
	bestEffortAppendEvent(ctx, s.events, AppendEventInput{
		Type:        eventdomain.TypeWorkspaceCreated,
		Severity:    eventdomain.SeverityInfo,
		TenantID:    input.TenantID,
		UserID:      input.UserID,
		SessionID:   input.SessionID,
		WorkspaceID: record.ID,
		Actor:       eventdomain.EventActor{Type: "user", ID: input.UserID},
		Subject:     eventdomain.EventSubject{Type: "workspace", ID: record.ID},
		Payload: map[string]any{
			"sandbox_profile_id": profile.ProfileID,
			"sandbox_service_id": s.sandboxServiceID,
		},
	})
	return protoRecord, nil
}

func (s *WorkspaceService) selectSandboxProfile(ctx context.Context, input CreateSessionWorkspaceInput) (*sandboxpolicydomain.ResolveWorkspaceProfileResult, error) {
	if s.profileResolver == nil {
		return nil, status.Error(codes.FailedPrecondition, "sandbox policy resolver is not configured")
	}
	available, err := s.profileCatalog.AvailableProfileIDs(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.profileResolver.ResolveWorkspaceProfile(ctx, sandboxpolicydomain.ResolveWorkspaceProfileRequest{
		TenantID:            input.TenantID,
		UserID:              input.UserID,
		RequestedProfileID:  input.RequestedProfileID,
		AvailableProfileIDs: available,
	})
	if err != nil {
		return nil, mapSandboxPolicyError(err)
	}
	return result, nil
}

type SandboxProfileCatalog struct {
	client   sandboxclient.WorkspaceHostClient
	cache    SandboxProfileCache
	cacheTTL time.Duration
}

type SandboxProfileCache struct {
	mu        sync.RWMutex
	expiresAt time.Time
	profiles  map[string]struct{}
}

func NewSandboxProfileCatalog(client sandboxclient.WorkspaceHostClient, cacheTTL time.Duration) *SandboxProfileCatalog {
	return &SandboxProfileCatalog{client: client, cacheTTL: cacheTTL}
}

func (c *SandboxProfileCatalog) AvailableProfileIDs(ctx context.Context) (map[string]struct{}, error) {
	if profiles, ok := c.cache.Get(time.Now()); ok {
		return profiles, nil
	}
	descriptor, err := c.client.GetCapabilityDescriptor(ctx)
	if err != nil {
		return nil, err
	}
	profiles := availableSandboxProfiles(descriptor)
	c.cache.Store(profiles, time.Now().Add(c.cacheTTL))
	return cloneProfileSet(profiles), nil
}

func (c *SandboxProfileCatalog) EnsureAvailable(ctx context.Context, profileID string) error {
	profiles, err := c.AvailableProfileIDs(ctx)
	if err != nil {
		return err
	}
	if _, ok := profiles[profileID]; !ok {
		return status.Errorf(codes.FailedPrecondition, "sandbox profile unavailable: %s", profileID)
	}
	return nil
}

func (c *SandboxProfileCache) Get(now time.Time) (map[string]struct{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.profiles == nil || now.After(c.expiresAt) {
		return nil, false
	}
	return cloneProfileSet(c.profiles), true
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

func cloneProfileSet(profiles map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(profiles))
	for profileID := range profiles {
		out[profileID] = struct{}{}
	}
	return out
}

func mapSandboxPolicyError(err error) error {
	switch {
	case errors.Is(err, sandboxpolicydomain.ErrNoProfileSelected):
		return status.Error(codes.FailedPrecondition, "no sandbox profile selected")
	case errors.Is(err, sandboxpolicydomain.ErrProfileNotAllowed):
		return status.Error(codes.PermissionDenied, "sandbox profile not allowed by policy")
	case errors.Is(err, sandboxpolicydomain.ErrProfileUnavailable):
		return status.Error(codes.FailedPrecondition, "sandbox profile unavailable")
	default:
		return err
	}
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
	ctx, span := telemetry.Start(ctx, "agent-control-plane/service", telemetry.SpanResourceExport)
	defer span.End()
	var resourceAuthorityServiceID string
	span.SetAttributes(attribute.String(telemetry.AttrOperation, "resource.export"))
	if s.resourceService == nil {
		err := status.Error(codes.FailedPrecondition, "resource service is not configured")
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		recordResourceTransfer(ctx, "export", telemetry.StatusError, "", 0)
		return nil, err
	}
	record, err := s.getWorkspaceRecordForView(ctx, sessionID)
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		recordResourceTransfer(ctx, "export", statusValue(err), "", 0)
		return nil, err
	}
	span.SetAttributes(attribute.String(telemetry.AttrSandboxProfileID, record.CurrentHost.GetSandboxProfileId()))
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
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		recordResourceTransfer(ctx, "export", statusValue(err), "", 0)
		return nil, err
	}
	if err := validateWorkspacePathRef(resp.GetSource(), record.CurrentHost); err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		recordResourceTransfer(ctx, "export", telemetry.StatusError, "", 0)
		return nil, err
	}
	resourceRef := resp.GetResource()
	if resourceRef == nil {
		err := status.Error(codes.Internal, "sandbox export returned empty resource ref")
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		recordResourceTransfer(ctx, "export", telemetry.StatusError, "", 0)
		return nil, err
	}
	resourceAuthorityServiceID = resourceRef.GetAuthorityServiceId()
	if resourceRef.GetAuthorityServiceId() != record.CurrentHost.GetServiceId() {
		err := status.Errorf(codes.Internal, "sandbox export returned resource for unexpected authority_service_id: %s", resourceRef.GetAuthorityServiceId())
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		recordResourceTransfer(ctx, "export", telemetry.StatusError, resourceAuthorityServiceID, 0)
		return nil, err
	}
	registered, err := s.resourceService.RegisterRecord(ctx, &resourcev1.RegisterResourceRequest{
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
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		recordResourceTransfer(ctx, "export", statusValue(err), resourceAuthorityServiceID, 0)
		return nil, err
	}
	span.SetAttributes(
		attribute.String(telemetry.AttrResourceAuthorityServiceID, registered.GetRef().GetAuthorityServiceId()),
		attribute.String(telemetry.AttrResourceMimeType, registered.GetRef().GetMimeType()),
		attribute.Int64(telemetry.AttrResourceSizeBytes, registered.GetRef().GetSizeBytes()),
		attribute.String(telemetry.AttrStatus, telemetry.StatusOK),
	)
	recordResourceTransfer(ctx, "export", telemetry.StatusOK, registered.GetRef().GetAuthorityServiceId(), registered.GetRef().GetSizeBytes())
	bestEffortAppendEvent(ctx, s.events, AppendEventInput{
		Type:        eventdomain.TypeResourceExportedFromWorkspace,
		Severity:    eventdomain.SeverityInfo,
		UserID:      ownerUserID,
		SessionID:   record.SessionID,
		WorkspaceID: record.ID,
		ResourceID:  registered.GetRef().GetId(),
		Actor:       eventdomain.EventActor{Type: "user", ID: ownerUserID},
		Subject:     eventdomain.EventSubject{Type: "resource", ID: registered.GetRef().GetId()},
		Payload: map[string]any{
			"mime_type":            registered.GetRef().GetMimeType(),
			"size_bytes":           registered.GetRef().GetSizeBytes(),
			"authority_service_id": registered.GetRef().GetAuthorityServiceId(),
		},
	})
	return registered, nil
}

func (s *WorkspaceService) ImportResourceToSessionWorkspace(ctx context.Context, sessionID string, userID string, resourceID string, destinationPath string, conflictPolicy sandboxv1.ImportConflictPolicy) (*sandboxv1.ImportResourceToWorkspaceResponse, error) {
	ctx, span := telemetry.Start(ctx, "agent-control-plane/service", telemetry.SpanResourceImport)
	defer span.End()
	var resourceAuthorityServiceID string
	span.SetAttributes(attribute.String(telemetry.AttrOperation, "resource.import"))
	if s.resourceGateway == nil {
		err := status.Error(codes.FailedPrecondition, "resource gateway is not configured")
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		recordResourceTransfer(ctx, "import", telemetry.StatusError, "", 0)
		return nil, err
	}
	record, err := s.getWorkspaceRecordForView(ctx, sessionID)
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		recordResourceTransfer(ctx, "import", statusValue(err), "", 0)
		return nil, err
	}
	span.SetAttributes(attribute.String(telemetry.AttrSandboxProfileID, record.CurrentHost.GetSandboxProfileId()))
	ownerUserID := userIDOrRecord(userID, record.OwnerUserID)
	resourceStream, err := s.resourceGateway.OpenResourceForTransfer(ctx, resourceID, ownerUserID)
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		recordResourceTransfer(ctx, "import", statusValue(err), "", 0)
		return nil, err
	}
	resourceRecord := resourceStream.Record()
	ref := resourceRecord.GetRef()
	resourceAuthorityServiceID = ref.GetAuthorityServiceId()
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
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		recordResourceTransfer(ctx, "import", statusValue(err), resourceAuthorityServiceID, 0)
		return nil, err
	}
	if err := validateWorkspacePathRef(resp.GetPath(), record.CurrentHost); err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		recordResourceTransfer(ctx, "import", telemetry.StatusError, resourceAuthorityServiceID, 0)
		return nil, err
	}
	span.SetAttributes(
		attribute.String(telemetry.AttrResourceMimeType, resp.GetMimeType()),
		attribute.Int64(telemetry.AttrResourceSizeBytes, resp.GetSizeBytes()),
		attribute.String(telemetry.AttrStatus, telemetry.StatusOK),
	)
	recordResourceTransfer(ctx, "import", telemetry.StatusOK, resourceAuthorityServiceID, resp.GetSizeBytes())
	bestEffortAppendEvent(ctx, s.events, AppendEventInput{
		Type:        eventdomain.TypeResourceImportedToWorkspace,
		Severity:    eventdomain.SeverityInfo,
		UserID:      ownerUserID,
		SessionID:   record.SessionID,
		WorkspaceID: record.ID,
		ResourceID:  ref.GetId(),
		Actor:       eventdomain.EventActor{Type: "user", ID: ownerUserID},
		Subject:     eventdomain.EventSubject{Type: "workspace", ID: record.ID},
		Payload: map[string]any{
			"mime_type":  resp.GetMimeType(),
			"size_bytes": resp.GetSizeBytes(),
		},
	})
	return resp, nil
}

func (s *WorkspaceService) ExecSessionWorkspaceCommand(ctx context.Context, input ExecSessionWorkspaceCommandInput) (*sandboxv1.ExecWorkspaceCommandResponse, error) {
	ctx, span := telemetry.Start(ctx, "agent-control-plane/workspace", telemetry.SpanWorkspaceExec)
	defer span.End()
	startedAt := time.Now()
	var sandboxProfileID string
	span.SetAttributes(
		attribute.String(telemetry.AttrOperation, "workspace.exec"),
		attribute.String(telemetry.AttrExecCommandName, telemetry.SafeCommandName(input.Command)),
		attribute.Int(telemetry.AttrExecArgCount, len(input.Args)),
	)
	if input.Command == "" {
		err := status.Error(codes.InvalidArgument, "command is required")
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusInvalid))
		recordWorkspaceExec(ctx, telemetry.StatusInvalid, sandboxProfileID, time.Since(startedAt))
		return nil, err
	}
	record, err := s.getWorkspaceRecordForView(ctx, input.SessionID)
	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		recordWorkspaceExec(ctx, statusValue(err), sandboxProfileID, time.Since(startedAt))
		return nil, err
	}
	sandboxProfileID = record.CurrentHost.GetSandboxProfileId()
	span.SetAttributes(attribute.String(telemetry.AttrSandboxProfileID, sandboxProfileID))
	userID := userIDOrRecord(input.UserID, record.OwnerUserID)
	if record.OwnerUserID != "" && userID != "" && record.OwnerUserID != userID {
		err := status.Error(codes.PermissionDenied, "workspace owner mismatch")
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusDenied))
		recordWorkspaceExec(ctx, telemetry.StatusDenied, sandboxProfileID, time.Since(startedAt))
		return nil, err
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
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, statusValue(err)))
		recordWorkspaceExec(ctx, statusValue(err), sandboxProfileID, time.Since(startedAt))
		appendWorkspaceExecFailedEvent(ctx, s.events, record, userID, input, err)
		return nil, err
	}
	if err := validateWorkspaceHostRef(resp.GetWorkspace(), record.CurrentHost); err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String(telemetry.AttrStatus, telemetry.StatusError))
		recordWorkspaceExec(ctx, telemetry.StatusError, sandboxProfileID, time.Since(startedAt))
		appendWorkspaceExecFailedEvent(ctx, s.events, record, userID, input, err)
		return nil, err
	}
	metricStatus := telemetry.StatusOK
	if resp.GetExitCode() != 0 {
		metricStatus = telemetry.StatusNonzeroExit
	}
	span.SetAttributes(
		attribute.Int(telemetry.AttrExecExitCode, int(resp.GetExitCode())),
		attribute.Bool(telemetry.AttrExecStdoutTruncated, resp.GetStdoutTruncated()),
		attribute.Bool(telemetry.AttrExecStderrTruncated, resp.GetStderrTruncated()),
		attribute.String(telemetry.AttrStatus, telemetry.StatusOK),
	)
	recordWorkspaceExec(ctx, metricStatus, sandboxProfileID, time.Since(startedAt))
	severity := eventdomain.SeverityInfo
	if resp.GetExitCode() != 0 {
		severity = eventdomain.SeverityWarning
	}
	bestEffortAppendEvent(ctx, s.events, AppendEventInput{
		Type:        eventdomain.TypeWorkspaceExecCompleted,
		Severity:    severity,
		UserID:      userID,
		SessionID:   record.SessionID,
		WorkspaceID: record.ID,
		Actor:       eventdomain.EventActor{Type: "user", ID: userID},
		Subject:     eventdomain.EventSubject{Type: "workspace", ID: record.ID},
		Payload: map[string]any{
			"exit_code":        resp.GetExitCode(),
			"stdout_truncated": resp.GetStdoutTruncated(),
			"stderr_truncated": resp.GetStderrTruncated(),
			"command_name":     telemetry.SafeCommandName(input.Command),
			"arg_count":        len(input.Args),
		},
	})
	return resp, nil
}

func appendWorkspaceExecFailedEvent(ctx context.Context, appender EventAppender, record workspacedomain.Record, userID string, input ExecSessionWorkspaceCommandInput, err error) {
	bestEffortAppendEvent(ctx, appender, AppendEventInput{
		Type:        eventdomain.TypeWorkspaceExecFailed,
		Severity:    eventdomain.SeverityError,
		UserID:      userID,
		SessionID:   record.SessionID,
		WorkspaceID: record.ID,
		Actor:       eventdomain.EventActor{Type: "user", ID: userID},
		Subject:     eventdomain.EventSubject{Type: "workspace", ID: record.ID},
		Payload: map[string]any{
			"error_code":   status.Code(err).String(),
			"command_name": telemetry.SafeCommandName(input.Command),
			"arg_count":    len(input.Args),
		},
	})
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
