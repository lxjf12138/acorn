package provider

import (
	"context"

	providerv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/provider/v1"
)

type Catalog interface {
	ListProviders(ctx context.Context) ([]*providerv1.ProviderManifest, error)
	GetProvider(ctx context.Context, providerID string) (*providerv1.ProviderManifest, error)
	GetProviderHealth(ctx context.Context, providerID string) (*providerv1.ProviderHealth, error)
}
