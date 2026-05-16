package resource

import (
	"context"
	"errors"

	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
)

var (
	ErrAlreadyExists    = errors.New("resource already exists")
	ErrInvalidResource  = errors.New("invalid resource")
	ErrResourceNotFound = errors.New("resource not found")
)

type ListFilter struct {
	OwnerUserID string
	SessionID   string
	Status      resourcev1.ResourceStatus
	Visibility  resourcev1.ResourceVisibility
}

type Catalog interface {
	Register(ctx context.Context, record *resourcev1.ResourceRecord) (*resourcev1.ResourceRecord, error)
	Get(ctx context.Context, resourceID string) (*resourcev1.ResourceRecord, error)
	List(ctx context.Context, filter ListFilter) ([]*resourcev1.ResourceRecord, error)
}
