package eventing

import (
	"context"

	"github.com/lxjf12138/acorn/packages/core/events"
)

type NoopEmitter struct{}

func (NoopEmitter) Emit(context.Context, events.Event) {}
