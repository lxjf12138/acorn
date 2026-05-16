package testkit

import (
	"context"
	"sync"

	toolv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/tool/v1"
	toolcore "github.com/lxjf12138/acorn/packages/core/tool"
	toolprovidercore "github.com/lxjf12138/acorn/packages/core/toolprovider"
)

type FakeToolRouter struct {
	mu            sync.RWMutex
	toolProviders map[string]toolprovidercore.ToolProvider
}

func NewFakeToolRouter() *FakeToolRouter {
	return &FakeToolRouter{toolProviders: make(map[string]toolprovidercore.ToolProvider)}
}

func (f *FakeToolRouter) AddToolProvider(provider toolprovidercore.ToolProvider) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.toolProviders[provider.ID()] = provider
}

func (f *FakeToolRouter) CallTool(ctx context.Context, req *toolv1.ToolCallRequest) (*toolv1.ToolCallResult, error) {
	f.mu.RLock()
	provider, ok := f.toolProviders[req.GetTargetServiceId()]
	f.mu.RUnlock()
	if !ok {
		return nil, toolprovidercore.ErrToolProviderNotFound
	}
	return provider.CallTool(ctx, req)
}

var _ toolcore.Router = (*FakeToolRouter)(nil)
