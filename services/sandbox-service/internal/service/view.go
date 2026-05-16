package service

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	pathpkg "path"
	"sort"
	"strconv"

	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	pathdomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/path"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultListPageSize = int32(100)
	maxListPageSize     = int32(500)
	defaultPreviewBytes = int64(64 * 1024)
	maxPreviewBytes     = int64(1024 * 1024)
)

type WorkspaceViewService struct {
	sandboxv1.UnimplementedWorkspaceViewServiceServer

	serviceID           string
	store               workspacedomain.Store
	defaultListPageSize int32
	maxListPageSize     int32
	defaultPreviewBytes int64
	maxPreviewBytes     int64
}

func NewWorkspaceViewService(serviceID string, store workspacedomain.Store) *WorkspaceViewService {
	return &WorkspaceViewService{
		serviceID:           serviceID,
		store:               store,
		defaultListPageSize: defaultListPageSize,
		maxListPageSize:     maxListPageSize,
		defaultPreviewBytes: defaultPreviewBytes,
		maxPreviewBytes:     maxPreviewBytes,
	}
}

func (s *WorkspaceViewService) ListWorkspaceDir(ctx context.Context, req *sandboxv1.ListWorkspaceDirRequest) (*sandboxv1.ListWorkspaceDirResponse, error) {
	workspace, relPath, err := s.workspaceAndPath(ctx, req.GetServiceWorkspaceId(), req.GetPath(), true)
	if err != nil {
		return nil, err
	}
	absPath, err := pathdomain.LexicalWorkspacePath(workspace.RootPath, relPath)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid workspace path")
	}
	info, err := os.Lstat(absPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, status.Error(codes.NotFound, "workspace directory not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "stat workspace directory: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, status.Error(codes.PermissionDenied, "workspace directory view does not follow symlinks")
	}
	resolvedPath, err := pathdomain.ResolveExistingWorkspacePath(workspace.RootPath, absPath)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "workspace path escapes workspace root")
	}
	if !info.IsDir() {
		return nil, status.Error(codes.FailedPrecondition, "workspace path is not a directory")
	}

	entries, err := os.ReadDir(resolvedPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, status.Error(codes.NotFound, "workspace directory not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read workspace directory: %v", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	start, err := parsePageToken(req.GetPageToken())
	if err != nil {
		return nil, err
	}
	pageSize := clampPageSize(req.GetPageSize(), s.defaultListPageSize, s.maxListPageSize)
	if start > len(entries) {
		start = len(entries)
	}
	end := start + int(pageSize)
	if end > len(entries) {
		end = len(entries)
	}
	out := make([]*sandboxv1.WorkspaceDirEntry, 0, end-start)
	for _, entry := range entries[start:end] {
		childRel := joinRelativePath(relPath, entry.Name())
		childInfo, _ := entry.Info()
		kind := pathKindFromDirEntry(entry, childInfo)
		out = append(out, &sandboxv1.WorkspaceDirEntry{
			Name:       entry.Name(),
			Ref:        s.pathRef(workspace, childRel, kind),
			Kind:       kind,
			SizeBytes:  sizeFromInfo(childInfo),
			ModifiedAt: modifiedAtFromInfo(childInfo),
		})
	}
	nextPageToken := ""
	if end < len(entries) {
		nextPageToken = strconv.Itoa(end)
	}
	return &sandboxv1.ListWorkspaceDirResponse{
		Directory:     s.pathRef(workspace, relPath, sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_DIRECTORY),
		Entries:       out,
		NextPageToken: nextPageToken,
	}, nil
}

func (s *WorkspaceViewService) PreviewWorkspaceFile(ctx context.Context, req *sandboxv1.PreviewWorkspaceFileRequest) (*sandboxv1.PreviewWorkspaceFileResponse, error) {
	workspace, relPath, err := s.workspaceAndPath(ctx, req.GetServiceWorkspaceId(), req.GetPath(), false)
	if err != nil {
		return nil, err
	}
	absPath, err := pathdomain.LexicalWorkspacePath(workspace.RootPath, relPath)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid workspace path")
	}
	info, err := os.Lstat(absPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, status.Error(codes.NotFound, "workspace file not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "stat workspace file: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, status.Error(codes.PermissionDenied, "workspace file preview does not follow symlinks")
	}
	resolvedPath, err := pathdomain.ResolveExistingWorkspacePath(workspace.RootPath, absPath)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "workspace path escapes workspace root")
	}
	if info.IsDir() {
		return nil, status.Error(codes.FailedPrecondition, "workspace path is a directory")
	}

	limit := clampPreviewBytes(req.GetMaxBytes(), s.defaultPreviewBytes, s.maxPreviewBytes)
	file, err := os.Open(resolvedPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open workspace file: %v", err)
	}
	defer file.Close()

	preview, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read workspace file: %v", err)
	}
	truncated := int64(len(preview)) > limit
	if truncated {
		preview = preview[:limit]
	}
	return &sandboxv1.PreviewWorkspaceFileResponse{
		File:         s.pathRef(workspace, relPath, sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE),
		MimeType:     detectMimeType(relPath, preview),
		PreviewBytes: preview,
		Truncated:    truncated,
		SizeBytes:    info.Size(),
		ModifiedAt:   timestamppb.New(info.ModTime()),
	}, nil
}

func (s *WorkspaceViewService) workspaceAndPath(ctx context.Context, serviceWorkspaceID string, inputPath string, allowRoot bool) (workspacedomain.Workspace, string, error) {
	if serviceWorkspaceID == "" {
		return workspacedomain.Workspace{}, "", status.Error(codes.InvalidArgument, "service_workspace_id is required")
	}
	workspace, err := s.store.Get(ctx, serviceWorkspaceID)
	if errors.Is(err, workspacedomain.ErrNotFound) {
		return workspacedomain.Workspace{}, "", status.Error(codes.NotFound, "hosted workspace not found")
	}
	if err != nil {
		return workspacedomain.Workspace{}, "", status.Errorf(codes.Internal, "get hosted workspace: %v", err)
	}
	if workspace.RootPath == "" {
		return workspacedomain.Workspace{}, "", status.Error(codes.FailedPrecondition, "hosted workspace has no root path")
	}
	relPath, err := pathdomain.NormalizeWorkspacePath(inputPath, allowRoot)
	if err != nil {
		return workspacedomain.Workspace{}, "", status.Error(codes.InvalidArgument, "invalid workspace path")
	}
	return workspace, relPath, nil
}

func (s *WorkspaceViewService) pathRef(workspace workspacedomain.Workspace, relPath string, kind sandboxv1.WorkspacePathKind) *sandboxv1.WorkspacePathRef {
	return &sandboxv1.WorkspacePathRef{
		Workspace: &workspacev1.WorkspaceHostRef{
			ServiceId:          s.serviceID,
			ServiceWorkspaceId: workspace.ID,
			SandboxProfileId:   workspace.SandboxProfileID,
		},
		Path:        relPath,
		Kind:        kind,
		DisplayName: displayNameForPath(relPath),
	}
}

func parsePageToken(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(token)
	if err != nil || offset < 0 {
		return 0, status.Error(codes.InvalidArgument, "invalid page_token")
	}
	return offset, nil
}

func clampPageSize(requested int32, defaultValue int32, maxValue int32) int32 {
	if requested <= 0 {
		return defaultValue
	}
	if requested > maxValue {
		return maxValue
	}
	return requested
}

func clampPreviewBytes(requested int64, defaultValue int64, maxValue int64) int64 {
	if requested <= 0 {
		return defaultValue
	}
	if requested > maxValue {
		return maxValue
	}
	return requested
}

func pathKindFromDirEntry(entry os.DirEntry, info os.FileInfo) sandboxv1.WorkspacePathKind {
	if entry.Type()&os.ModeSymlink != 0 {
		return sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_SYMLINK
	}
	if info == nil {
		return sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_UNKNOWN
	}
	if info.IsDir() {
		return sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_DIRECTORY
	}
	if info.Mode().IsRegular() {
		return sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE
	}
	return sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_UNKNOWN
}

func sizeFromInfo(info os.FileInfo) int64 {
	if info == nil {
		return 0
	}
	return info.Size()
}

func modifiedAtFromInfo(info os.FileInfo) *timestamppb.Timestamp {
	if info == nil || info.ModTime().IsZero() {
		return nil
	}
	return timestamppb.New(info.ModTime())
}

func joinRelativePath(parent string, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

func detectMimeType(relPath string, preview []byte) string {
	if mimeType := mime.TypeByExtension(pathpkg.Ext(relPath)); mimeType != "" {
		return mimeType
	}
	if len(preview) == 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(preview)
}

func displayNameForPath(relPath string) string {
	if relPath == "" {
		return ""
	}
	return pathpkg.Base(relPath)
}
