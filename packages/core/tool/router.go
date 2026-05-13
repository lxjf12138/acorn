package tool

import (
	"context"

	toolv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/tool/v1"
)

type Router interface {
	CallTool(ctx context.Context, req *toolv1.ToolCallRequest) (*toolv1.ToolCallResult, error)
}
