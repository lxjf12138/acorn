package profile

import "errors"

var (
	ErrProfileNotFound       = errors.New("sandbox profile not found")
	ErrNoDefaultProfile      = errors.New("no default sandbox profile")
	ErrProfileDisabled       = errors.New("sandbox profile disabled")
	ErrCapabilityUnsupported = errors.New("sandbox profile capability unsupported")
)
