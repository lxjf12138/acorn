package event

import (
	"context"

	eventv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/event/v1"
)

type Bus interface {
	Publish(ctx context.Context, event *eventv1.Event) error
}
