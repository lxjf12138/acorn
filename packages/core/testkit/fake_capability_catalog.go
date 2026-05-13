package testkit

import (
	"context"
	"sync"

	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
	capabilitycore "github.com/lxjf12138/acorn/packages/core/capability"
	"google.golang.org/protobuf/proto"
)

type FakeCapabilityCatalog struct {
	mu       sync.RWMutex
	services map[string]*capabilityv1.CapabilityService
}

func NewFakeCapabilityCatalog() *FakeCapabilityCatalog {
	return &FakeCapabilityCatalog{services: make(map[string]*capabilityv1.CapabilityService)}
}

func (f *FakeCapabilityCatalog) AddService(service *capabilityv1.CapabilityService) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.services[service.GetId()] = proto.Clone(service).(*capabilityv1.CapabilityService)
}

func (f *FakeCapabilityCatalog) ListServices(context.Context) ([]*capabilityv1.CapabilityService, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]*capabilityv1.CapabilityService, 0, len(f.services))
	for _, service := range f.services {
		out = append(out, proto.Clone(service).(*capabilityv1.CapabilityService))
	}
	return out, nil
}

func (f *FakeCapabilityCatalog) GetService(_ context.Context, serviceID string) (*capabilityv1.CapabilityService, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	service, ok := f.services[serviceID]
	if !ok {
		return nil, capabilitycore.ErrServiceNotFound
	}
	return proto.Clone(service).(*capabilityv1.CapabilityService), nil
}

var _ capabilitycore.Catalog = (*FakeCapabilityCatalog)(nil)
