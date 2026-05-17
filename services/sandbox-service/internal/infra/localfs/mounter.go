package localfs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/attachment"
	workspacestore "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacestore"
)

var _ attachment.WorkspaceMounter = (*WorkspaceStore)(nil)

func (s *WorkspaceStore) Prepare(_ context.Context, req attachment.PrepareRequest) (*attachment.WorkspaceAttachment, error) {
	if req.WorkspaceID == "" {
		return nil, attachment.ErrWorkspaceNotFound
	}
	if req.Target.Kind != attachment.TargetLocalProcess {
		return nil, attachment.ErrUnsupportedTarget
	}
	localPath, err := s.resolvedWorkspaceRootForAttachment(req.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return &attachment.WorkspaceAttachment{
		ID:          "att_" + req.WorkspaceID,
		WorkspaceID: req.WorkspaceID,
		Kind:        attachment.KindLocalPath,
		LocalPath:   localPath,
		ReadOnly:    req.ReadOnly || req.Target.ReadOnly,
		Metadata: map[string]string{
			"store_kind": s.Kind(),
		},
	}, nil
}

func (s *WorkspaceStore) Release(context.Context, *attachment.WorkspaceAttachment) error {
	return nil
}

func (s *WorkspaceStore) resolvedWorkspaceRootForAttachment(workspaceID string) (string, error) {
	root, err := s.workspaceRoot(workspaceID)
	if errors.Is(err, workspacestore.ErrWorkspaceNotFound) {
		return "", attachment.ErrWorkspaceNotFound
	}
	if err != nil {
		return "", err
	}
	info, err := os.Stat(root)
	if errors.Is(err, os.ErrNotExist) {
		return "", attachment.ErrWorkspaceNotFound
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", attachment.ErrAttachmentNotReady
	}
	resolvedBase, err := filepath.EvalSymlinks(s.baseDir)
	if errors.Is(err, os.ErrNotExist) {
		return "", attachment.ErrAttachmentNotReady
	}
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if errors.Is(err, os.ErrNotExist) {
		return "", attachment.ErrWorkspaceNotFound
	}
	if err != nil {
		return "", err
	}
	resolvedBase, err = filepath.Abs(resolvedBase)
	if err != nil {
		return "", err
	}
	resolvedRoot, err = filepath.Abs(resolvedRoot)
	if err != nil {
		return "", err
	}
	if resolvedRoot != resolvedBase && !strings.HasPrefix(resolvedRoot, resolvedBase+string(os.PathSeparator)) {
		return "", attachment.ErrAttachmentNotReady
	}
	return resolvedRoot, nil
}
