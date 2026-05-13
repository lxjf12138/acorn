package capability

import (
	"context"

	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
)

type Catalog interface {
	ListServices(ctx context.Context) ([]*capabilityv1.CapabilityService, error)
	GetService(ctx context.Context, serviceID string) (*capabilityv1.CapabilityService, error)
}
