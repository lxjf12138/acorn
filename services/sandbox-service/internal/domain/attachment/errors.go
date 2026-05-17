package attachment

import "errors"

var (
	ErrWorkspaceNotFound  = errors.New("workspace not found")
	ErrUnsupportedTarget  = errors.New("unsupported attachment target")
	ErrAttachmentNotReady = errors.New("workspace attachment not ready")
)
