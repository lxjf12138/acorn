package localfs

import (
	"context"
	"errors"
	"io"
	"os"
	pathpkg "path"
	"sort"
	"strconv"
	"time"

	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	workspacestore "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacestore"
)

const (
	Kind = "localfs"

	defaultListPageSize = int32(100)
	maxListPageSize     = int32(500)
	defaultPreviewBytes = int64(64 * 1024)
	maxPreviewBytes     = int64(1024 * 1024)
)

type Config struct {
	BaseDir string

	DefaultListPageSize int32
	MaxListPageSize     int32
	DefaultPreviewBytes int64
	MaxPreviewBytes     int64
}

type WorkspaceStore struct {
	baseDir string

	defaultListPageSize int32
	maxListPageSize     int32
	defaultPreviewBytes int64
	maxPreviewBytes     int64
}

func NewWorkspaceStore(cfg Config) (*WorkspaceStore, error) {
	if cfg.BaseDir == "" {
		return nil, workspacestore.ErrWorkspaceNotReady
	}
	store := &WorkspaceStore{
		baseDir:             cfg.BaseDir,
		defaultListPageSize: valueOrDefaultInt32(cfg.DefaultListPageSize, defaultListPageSize),
		maxListPageSize:     valueOrDefaultInt32(cfg.MaxListPageSize, maxListPageSize),
		defaultPreviewBytes: valueOrDefaultInt64(cfg.DefaultPreviewBytes, defaultPreviewBytes),
		maxPreviewBytes:     valueOrDefaultInt64(cfg.MaxPreviewBytes, maxPreviewBytes),
	}
	return store, nil
}

func (s *WorkspaceStore) Kind() string {
	return Kind
}

func (s *WorkspaceStore) CreateBackingWorkspace(_ context.Context, req workspacestore.CreateBackingWorkspaceRequest) (*workspacestore.BackingWorkspace, error) {
	if req.WorkspaceID == "" {
		return nil, workspacestore.ErrWorkspaceNotFound
	}
	root, err := s.workspaceRoot(req.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &workspacestore.BackingWorkspace{
		ID:               req.WorkspaceID,
		StoreKind:        s.Kind(),
		StoreWorkspaceID: req.WorkspaceID,
		SandboxProfileID: req.SandboxProfileID,
		Status:           workspacev1.WorkspaceStatus_WORKSPACE_STATUS_ACTIVE,
		CreatedAt:        now,
		UpdatedAt:        now,
		MetadataJSON:     append([]byte(nil), req.MetadataJSON...),
	}, nil
}

func (s *WorkspaceStore) DeleteBackingWorkspace(_ context.Context, workspaceID string) error {
	if workspaceID == "" {
		return workspacestore.ErrWorkspaceNotFound
	}
	root, err := s.workspaceRoot(workspaceID)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	return nil
}

func (s *WorkspaceStore) ListDir(_ context.Context, req workspacestore.ListDirRequest) (*workspacestore.DirListing, error) {
	relPath, err := normalizeWorkspacePath(req.Path, true)
	if err != nil {
		return nil, err
	}
	resolvedPath, info, err := s.resolvedPath(req.WorkspaceID, relPath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, workspacestore.ErrSymlinkNotAllowed
	}
	if !info.IsDir() {
		return nil, workspacestore.ErrPathNotDirectory
	}
	entries, err := os.ReadDir(resolvedPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, workspacestore.ErrPathNotFound
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	start, err := parsePageToken(req.PageToken)
	if err != nil {
		return nil, err
	}
	pageSize := clampPageSize(req.PageSize, s.defaultListPageSize, s.maxListPageSize)
	if start > len(entries) {
		start = len(entries)
	}
	end := start + int(pageSize)
	if end > len(entries) {
		end = len(entries)
	}
	out := make([]workspacestore.PathInfo, 0, end-start)
	for _, entry := range entries[start:end] {
		childRel := joinRelativePath(relPath, entry.Name())
		childInfo, _ := entry.Info()
		out = append(out, pathInfo(childRel, entry.Name(), pathKindFromDirEntry(entry, childInfo), childInfo))
	}
	nextPageToken := ""
	if end < len(entries) {
		nextPageToken = strconv.Itoa(end)
	}
	return &workspacestore.DirListing{
		Directory: pathInfo(relPath, displayNameForPath(relPath), sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_DIRECTORY, info),
		Entries:   out,
		NextToken: nextPageToken,
	}, nil
}

func (s *WorkspaceStore) PreviewFile(_ context.Context, req workspacestore.PreviewFileRequest) (*workspacestore.FilePreview, error) {
	relPath, err := normalizeWorkspacePath(req.Path, false)
	if err != nil {
		return nil, err
	}
	resolvedPath, info, err := s.resolvedPath(req.WorkspaceID, relPath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, workspacestore.ErrSymlinkNotAllowed
	}
	if info.IsDir() {
		return nil, workspacestore.ErrPathIsDirectory
	}
	if !info.Mode().IsRegular() {
		return nil, workspacestore.ErrPathNotRegularFile
	}
	limit := clampPreviewBytes(req.MaxBytes, s.defaultPreviewBytes, s.maxPreviewBytes)
	file, err := os.Open(resolvedPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	preview, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	truncated := int64(len(preview)) > limit
	if truncated {
		preview = preview[:limit]
	}
	return &workspacestore.FilePreview{
		File:      pathInfo(relPath, displayNameForPath(relPath), sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE, info),
		MimeType:  detectMimeType(relPath, preview),
		Bytes:     preview,
		Truncated: truncated,
	}, nil
}

func (s *WorkspaceStore) StatPath(_ context.Context, req workspacestore.StatPathRequest) (*workspacestore.PathInfo, error) {
	relPath, err := normalizeWorkspacePath(req.Path, true)
	if err != nil {
		return nil, err
	}
	_, info, err := s.resolvedPath(req.WorkspaceID, relPath)
	if err != nil {
		return nil, err
	}
	kind := sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_UNKNOWN
	if info.Mode()&os.ModeSymlink != 0 {
		kind = sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_SYMLINK
	} else if info.IsDir() {
		kind = sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_DIRECTORY
	} else if info.Mode().IsRegular() {
		kind = sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE
	}
	out := pathInfo(relPath, displayNameForPath(relPath), kind, info)
	return &out, nil
}

func (s *WorkspaceStore) ExportPath(_ context.Context, req workspacestore.ExportPathRequest) (*workspacestore.ExportedPath, error) {
	relPath, err := normalizeWorkspacePath(req.Path, false)
	if err != nil {
		return nil, err
	}
	resolvedPath, info, err := s.resolvedPath(req.WorkspaceID, relPath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, workspacestore.ErrSymlinkNotAllowed
	}
	if info.IsDir() {
		return nil, workspacestore.ErrPathIsDirectory
	}
	if !info.Mode().IsRegular() {
		return nil, workspacestore.ErrPathNotRegularFile
	}
	source := pathInfo(relPath, displayNameForPath(relPath), sandboxv1.WorkspacePathKind_WORKSPACE_PATH_KIND_FILE, info)
	return &workspacestore.ExportedPath{
		Source:    source,
		MimeType:  detectExportMimeType(relPath),
		SizeBytes: info.Size(),
		Open: func(context.Context) (io.ReadCloser, error) {
			return os.Open(resolvedPath)
		},
	}, nil
}

func (s *WorkspaceStore) resolvedPath(workspaceID string, relPath string) (string, os.FileInfo, error) {
	root, err := s.workspaceRoot(workspaceID)
	if err != nil {
		return "", nil, err
	}
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return "", nil, workspacestore.ErrWorkspaceNotReady
	} else if err != nil {
		return "", nil, err
	}
	absPath, err := lexicalWorkspacePath(root, relPath)
	if err != nil {
		return "", nil, err
	}
	info, err := os.Lstat(absPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil, workspacestore.ErrPathNotFound
	}
	if err != nil {
		return "", nil, err
	}
	resolvedPath, err := resolveExistingWorkspacePath(root, absPath)
	if err != nil {
		return "", nil, err
	}
	return resolvedPath, info, nil
}

func (s *WorkspaceStore) workspaceRoot(workspaceID string) (string, error) {
	if workspaceID == "" {
		return "", workspacestore.ErrWorkspaceNotFound
	}
	return lexicalWorkspacePath(s.baseDir, workspaceID)
}

func parsePageToken(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(token)
	if err != nil || offset < 0 {
		return 0, workspacestore.ErrInvalidPath
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

func pathInfo(relPath string, name string, kind sandboxv1.WorkspacePathKind, info os.FileInfo) workspacestore.PathInfo {
	out := workspacestore.PathInfo{
		Path: relPath,
		Name: name,
		Kind: kind,
	}
	if info != nil {
		out.SizeBytes = info.Size()
		out.ModifiedAt = info.ModTime()
	}
	return out
}

func joinRelativePath(parent string, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

func displayNameForPath(relPath string) string {
	if relPath == "" {
		return ""
	}
	return pathpkg.Base(relPath)
}

func valueOrDefaultInt32(value int32, defaultValue int32) int32 {
	if value <= 0 {
		return defaultValue
	}
	return value
}

func valueOrDefaultInt64(value int64, defaultValue int64) int64 {
	if value <= 0 {
		return defaultValue
	}
	return value
}
