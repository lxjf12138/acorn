package testkit

import (
	"context"
	"sync"

	signalv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/signal/v1"
	signalcore "github.com/lxjf12138/acorn/packages/core/signal"
	"google.golang.org/protobuf/proto"
)

type FakeSignalIngest struct {
	mu      sync.RWMutex
	signals []*signalv1.Signal
}

func NewFakeSignalIngest() *FakeSignalIngest {
	return &FakeSignalIngest{}
}

func (f *FakeSignalIngest) Emit(ctx context.Context, sig *signalv1.Signal) error {
	_ = ctx
	f.mu.Lock()
	defer f.mu.Unlock()
	f.signals = append(f.signals, proto.Clone(sig).(*signalv1.Signal))
	return nil
}

func (f *FakeSignalIngest) Signals() []*signalv1.Signal {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]*signalv1.Signal, 0, len(f.signals))
	for _, sig := range f.signals {
		out = append(out, proto.Clone(sig).(*signalv1.Signal))
	}
	return out
}

func (f *FakeSignalIngest) Clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.signals = nil
}

var _ signalcore.Ingest = (*FakeSignalIngest)(nil)
