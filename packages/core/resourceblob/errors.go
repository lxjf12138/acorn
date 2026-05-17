package resourceblob

import "errors"

var (
	ErrInvalidResourceID = errors.New("invalid resource id")
	ErrAlreadyExists     = errors.New("resource blob already exists")
	ErrNotFound          = errors.New("resource blob not found")
	ErrStoreNotReady     = errors.New("resource blob store not ready")
)
