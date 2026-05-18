package event

import "errors"

var (
	ErrEventIDRequired   = errors.New("event id required")
	ErrEventTypeRequired = errors.New("event type required")
	ErrInvalidLimit      = errors.New("invalid event list limit")
)
