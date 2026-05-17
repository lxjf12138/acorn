package workspacelease

import "errors"

var (
	ErrWorkspaceIDRequired = errors.New("workspace id required")
	ErrInvalidMode         = errors.New("invalid workspace lease mode")
	ErrWorkspaceBusy       = errors.New("workspace is busy")
	ErrLeaseNotFound       = errors.New("workspace lease not found")
)
