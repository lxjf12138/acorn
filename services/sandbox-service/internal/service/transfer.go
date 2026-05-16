package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"os"
	pathpkg "path"
	"time"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	exporteddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/exportedresource"
	pathdomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/path"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WorkspaceTransferService struct {
	sandboxv1.UnimplementedWorkspaceTransferServiceServer

	serviceID      string
	workspaceStore workspacedomain.Store
	exportStore    exporteddomain.Store
}

func NewWorkspaceTransferService(serviceID string, workspaceStore workspacedomain.Store, exportStore exporteddomain.Store) *WorkspaceTransferService {
	return &WorkspaceTransferService{
		serviceID:      serviceID,
		workspaceStore: workspaceStore,
		exportStore:    exportStore,
	}
}

func (s *WorkspaceTransferService) ExportWorkspacePath(ctx context.Context, req *sandboxv1.ExportWorkspacePathRequest) (*sandboxv1.ExportWorkspacePathResponse, error) {
	workspace, relPath, resolvedPath, info, err := s.exportableFile(ctx, req.GetServiceWorkspaceId(), req.GetPath())
	if err != nil {
		return nil, err
	}

	resourceID := newExportedResourceID()
	name := req.GetResourceName()
	if name == "" {
		name = pathpkg.Base(relPath)
	}
	mimeType := req.GetMimeType()
	if mimeType == "" {
		mimeType = detectExportMimeType(relPath)
	}

	if _, err := s.exportStore.Create(ctx, exporteddomain.Record{
		ResourceID:         resourceID,
		ServiceWorkspaceID: workspace.ID,
		WorkspacePath:      relPath,
		Name:               name,
		MimeType:           mimeType,
		SizeBytes:          info.Size(),
	}); err != nil {
		if errors.Is(err, exporteddomain.ErrAlreadyExists) {
			return nil, status.Error(codes.AlreadyExists, "exported resource already exists")
		}
		return nil, status.Errorf(codes.Internal, "create exported resource record: %v", err)
	}

	_ = resolvedPath
	return &sandboxv1.ExportWorkspacePathResponse{
		Source: &sandboxv1.WorkspacePathRef{
			Workspace: &workspacev1.WorkspaceHostRef{
				ServiceId:          s.serviceID,
				ServiceWorkspaceId: workspace.ID,
				SandboxProfileId:   workspace.SandboxProfileID,
			},
			Path:        relPath,
			Kind:        sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE,
			DisplayName: pathpkg.Base(relPath),
		},
		Resource: &resourcev1.ResourceRef{
			Id:                 resourceID,
			AuthorityServiceId: s.serviceID,
			Name:               name,
			MimeType:           mimeType,
			SizeBytes:          info.Size(),
			MetadataJson:       append([]byte(nil), req.GetMetadataJson()...),
		},
	}, nil
}

func (s *WorkspaceTransferService) exportableFile(ctx context.Context, serviceWorkspaceID string, inputPath string) (workspacedomain.Workspace, string, string, os.FileInfo, error) {
	if serviceWorkspaceID == "" {
		return workspacedomain.Workspace{}, "", "", nil, status.Error(codes.InvalidArgument, "service_workspace_id is required")
	}
	workspace, err := s.workspaceStore.Get(ctx, serviceWorkspaceID)
	if errors.Is(err, workspacedomain.ErrNotFound) {
		return workspacedomain.Workspace{}, "", "", nil, status.Error(codes.NotFound, "hosted workspace not found")
	}
	if err != nil {
		return workspacedomain.Workspace{}, "", "", nil, status.Errorf(codes.Internal, "get hosted workspace: %v", err)
	}
	if workspace.RootPath == "" {
		return workspacedomain.Workspace{}, "", "", nil, status.Error(codes.FailedPrecondition, "hosted workspace has no root path")
	}
	relPath, err := pathdomain.NormalizeWorkspacePath(inputPath, false)
	if err != nil {
		return workspacedomain.Workspace{}, "", "", nil, status.Error(codes.InvalidArgument, "invalid workspace path")
	}
	absPath, err := pathdomain.LexicalWorkspacePath(workspace.RootPath, relPath)
	if err != nil {
		return workspacedomain.Workspace{}, "", "", nil, status.Error(codes.InvalidArgument, "invalid workspace path")
	}
	info, err := os.Lstat(absPath)
	if errors.Is(err, os.ErrNotExist) {
		return workspacedomain.Workspace{}, "", "", nil, status.Error(codes.NotFound, "workspace file not found")
	}
	if err != nil {
		return workspacedomain.Workspace{}, "", "", nil, status.Errorf(codes.Internal, "stat workspace file: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return workspacedomain.Workspace{}, "", "", nil, status.Error(codes.PermissionDenied, "workspace export does not follow symlinks")
	}
	resolvedPath, err := pathdomain.ResolveExistingWorkspacePath(workspace.RootPath, absPath)
	if err != nil {
		return workspacedomain.Workspace{}, "", "", nil, status.Error(codes.PermissionDenied, "workspace path escapes workspace root")
	}
	if info.IsDir() {
		return workspacedomain.Workspace{}, "", "", nil, status.Error(codes.FailedPrecondition, "workspace path is a directory")
	}
	if !info.Mode().IsRegular() {
		return workspacedomain.Workspace{}, "", "", nil, status.Error(codes.FailedPrecondition, "workspace path is not a regular file")
	}
	return workspace, relPath, resolvedPath, info, nil
}

func detectExportMimeType(relPath string) string {
	if mimeType := mime.TypeByExtension(pathpkg.Ext(relPath)); mimeType != "" {
		return mimeType
	}
	return "application/octet-stream"
}

func newExportedResourceID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "res_" + hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("res_%d", time.Now().UnixNano())
}
