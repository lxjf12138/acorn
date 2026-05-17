package backend

import "errors"

var (
	ErrInvalidRequest        = errors.New("invalid sandbox backend request")
	ErrUnsupportedAttachment = errors.New("unsupported workspace attachment")
	ErrAttachmentNotReady    = errors.New("workspace attachment not ready")
	ErrInvalidCWD            = errors.New("invalid workspace cwd")
	ErrExecTimeout           = errors.New("sandbox exec timeout")
	ErrExecStart             = errors.New("sandbox exec start failed")
)
