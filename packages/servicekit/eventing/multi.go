package eventing

import (
	"context"

	"github.com/lxjf12138/acorn/packages/core/events"
)

type MultiEmitter struct {
	emitters []events.Emitter
}

func NewMultiEmitter(emitters ...events.Emitter) MultiEmitter {
	filtered := make([]events.Emitter, 0, len(emitters))
	for _, emitter := range emitters {
		if emitter != nil {
			filtered = append(filtered, emitter)
		}
	}
	return MultiEmitter{emitters: filtered}
}

func (m MultiEmitter) Emit(ctx context.Context, event events.Event) {
	for _, emitter := range m.emitters {
		emitter.Emit(ctx, event)
	}
}
