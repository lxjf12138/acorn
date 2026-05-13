package testkit

import (
	"context"
	"sync"

	eventv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/event/v1"
	eventcore "github.com/lxjf12138/acorn/packages/core/event"
	"google.golang.org/protobuf/proto"
)

type FakeEventBus struct {
	mu     sync.RWMutex
	events []*eventv1.Event
}

func NewFakeEventBus() *FakeEventBus {
	return &FakeEventBus{}
}

func (f *FakeEventBus) Publish(ctx context.Context, event *eventv1.Event) error {
	_ = ctx
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, proto.Clone(event).(*eventv1.Event))
	return nil
}

func (f *FakeEventBus) Events() []*eventv1.Event {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]*eventv1.Event, 0, len(f.events))
	for _, event := range f.events {
		out = append(out, proto.Clone(event).(*eventv1.Event))
	}
	return out
}

func (f *FakeEventBus) Clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = nil
}

var _ eventcore.Bus = (*FakeEventBus)(nil)
