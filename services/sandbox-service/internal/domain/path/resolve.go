package path

import (
	"os"
	"path/filepath"
	"strings"
)

func LexicalWorkspacePath(root string, rel string) (string, error) {
	if root == "" {
		return "", ErrInvalidPath
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
		return "", ErrInvalidPath
	}
	return absPath, nil
}

func ResolveExistingWorkspacePath(root string, absPath string) (string, error) {
	if root == "" {
		return "", ErrInvalidPath
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", err
	}
	if resolvedPath != resolvedRoot && !strings.HasPrefix(resolvedPath, resolvedRoot+string(os.PathSeparator)) {
		return "", ErrInvalidPath
	}
	return resolvedPath, nil
}
