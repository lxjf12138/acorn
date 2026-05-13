package testkit

import (
	"context"
	"sync"

	providerv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/provider/v1"
	providercore "github.com/lxjf12138/acorn/packages/core/provider"
	"google.golang.org/protobuf/proto"
)

type FakeProviderCatalog struct {
	mu        sync.RWMutex
	providers map[string]*providerv1.ProviderManifest
	healths   map[string]*providerv1.ProviderHealth
}

func NewFakeProviderCatalog() *FakeProviderCatalog {
	return &FakeProviderCatalog{
		providers: make(map[string]*providerv1.ProviderManifest),
		healths:   make(map[string]*providerv1.ProviderHealth),
	}
}

func (f *FakeProviderCatalog) AddProvider(manifest *providerv1.ProviderManifest, health *providerv1.ProviderHealth) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.providers[manifest.GetId()] = proto.Clone(manifest).(*providerv1.ProviderManifest)
	f.healths[manifest.GetId()] = proto.Clone(health).(*providerv1.ProviderHealth)
}

func (f *FakeProviderCatalog) ListProviders(context.Context) ([]*providerv1.ProviderManifest, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]*providerv1.ProviderManifest, 0, len(f.providers))
	for _, manifest := range f.providers {
		out = append(out, proto.Clone(manifest).(*providerv1.ProviderManifest))
	}
	return out, nil
}

func (f *FakeProviderCatalog) GetProvider(_ context.Context, providerID string) (*providerv1.ProviderManifest, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	manifest, ok := f.providers[providerID]
	if !ok {
		return nil, providercore.ErrProviderNotFound
	}
	return proto.Clone(manifest).(*providerv1.ProviderManifest), nil
}

func (f *FakeProviderCatalog) GetProviderHealth(_ context.Context, providerID string) (*providerv1.ProviderHealth, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	health, ok := f.healths[providerID]
	if !ok {
		return nil, providercore.ErrProviderNotFound
	}
	return proto.Clone(health).(*providerv1.ProviderHealth), nil
}

var _ providercore.Catalog = (*FakeProviderCatalog)(nil)
