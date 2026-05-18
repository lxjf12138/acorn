package execution

import "errors"

var (
	ErrRecordRequired   = errors.New("execution record is required")
	ErrIDRequired       = errors.New("execution id is required")
	ErrSessionRequired  = errors.New("execution session_id is required")
	ErrStatusRequired   = errors.New("execution status is required")
	ErrAlreadyExists    = errors.New("execution already exists")
	ErrNotFound         = errors.New("execution not found")
	ErrInvalidLimit     = errors.New("invalid execution list limit")
	ErrInvalidPageToken = errors.New("invalid execution page token")
)

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
