package provider

import (
	"context"

	providerv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/provider/v1"
	toolv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/tool/v1"
)

type Provider interface {
	ID() string
	Type() string
	Manifest(ctx context.Context) (*providerv1.ProviderManifest, error)
	Health(ctx context.Context) (*providerv1.ProviderHealth, error)
	ListTools(ctx context.Context) ([]*toolv1.ToolSpec, error)
	CallTool(ctx context.Context, req *toolv1.ToolCallRequest) (*toolv1.ToolCallResult, error)
}
