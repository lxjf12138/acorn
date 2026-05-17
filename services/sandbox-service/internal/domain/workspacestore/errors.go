package workspacestore

import "errors"

var (
	ErrInvalidPath          = errors.New("invalid workspace path")
	ErrPathNotFound         = errors.New("workspace path not found")
	ErrPathNotDirectory     = errors.New("workspace path is not a directory")
	ErrPathIsDirectory      = errors.New("workspace path is a directory")
	ErrPathNotRegularFile   = errors.New("workspace path is not a regular file")
	ErrSymlinkNotAllowed    = errors.New("workspace symlink is not allowed")
	ErrPathEscapesWorkspace = errors.New("workspace path escapes workspace root")
	ErrWorkspaceNotFound    = errors.New("workspace not found")
	ErrWorkspaceNotReady    = errors.New("workspace backing store not ready")
)
