package testkit

import (
	"context"
	"sync"

	toolv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/tool/v1"
	providercore "github.com/lxjf12138/acorn/packages/core/provider"
	toolcore "github.com/lxjf12138/acorn/packages/core/tool"
)

type FakeToolRouter struct {
	mu        sync.RWMutex
	providers map[string]providercore.Provider
}

func NewFakeToolRouter() *FakeToolRouter {
	return &FakeToolRouter{providers: make(map[string]providercore.Provider)}
}

func (f *FakeToolRouter) AddProvider(provider providercore.Provider) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.providers[provider.ID()] = provider
}

func (f *FakeToolRouter) CallTool(ctx context.Context, req *toolv1.ToolCallRequest) (*toolv1.ToolCallResult, error) {
	f.mu.RLock()
	provider, ok := f.providers[req.GetProviderId()]
	f.mu.RUnlock()
	if !ok {
		return nil, providercore.ErrProviderNotFound
	}
	return provider.CallTool(ctx, req)
}

var _ toolcore.Router = (*FakeToolRouter)(nil)
