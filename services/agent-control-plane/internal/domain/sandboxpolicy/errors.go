package sandboxpolicy

import "errors"

var (
	ErrNoProfileSelected  = errors.New("no sandbox profile selected")
	ErrProfileNotAllowed  = errors.New("sandbox profile not allowed by policy")
	ErrProfileUnavailable = errors.New("sandbox profile unavailable")
	ErrPolicyNotFound     = errors.New("sandbox policy not found")
)
