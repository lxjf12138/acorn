package localfs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	pathdomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/path"
	workspacestore "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacestore"
)

func lexicalWorkspacePath(root string, rel string) (string, error) {
	if root == "" {
		return "", workspacestore.ErrWorkspaceNotReady
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absPath := filepath.Join(absRoot, filepath.FromSlash(rel))
	absPath, err = filepath.Abs(absPath)
	if err != nil {
		return "", err
	}
	if absPath != absRoot && !strings.HasPrefix(absPath, absRoot+string(os.PathSeparator)) {
		return "", workspacestore.ErrInvalidPath
	}
	return absPath, nil
}

func resolveExistingWorkspacePath(root string, absPath string) (string, error) {
	if root == "" {
		return "", workspacestore.ErrWorkspaceNotReady
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if errors.Is(err, os.ErrNotExist) {
		return "", workspacestore.ErrWorkspaceNotReady
	}
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", workspacestore.ErrPathNotFound
	}
	if err != nil {
		return "", err
	}
	if resolvedPath != resolvedRoot && !strings.HasPrefix(resolvedPath, resolvedRoot+string(os.PathSeparator)) {
		return "", workspacestore.ErrPathEscapesWorkspace
	}
	return resolvedPath, nil
}

func normalizeWorkspacePath(input string, allowRoot bool) (string, error) {
	relPath, err := pathdomain.NormalizeWorkspacePath(input, allowRoot)
	if err != nil {
		return "", workspacestore.ErrInvalidPath
	}
	return relPath, nil
}
