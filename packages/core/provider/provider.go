package provider

import (
	"context"

	toolv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/tool/v1"
)

type Provider interface {
	ID() string
	Type() string
	ListTools(ctx context.Context) ([]*toolv1.ToolSpec, error)
	CallTool(ctx context.Context, req *toolv1.ToolCallRequest) (*toolv1.ToolCallResult, error)
}
