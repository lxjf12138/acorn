package events

import (
	"context"
	"time"
)

// Event is a structured Acorn domain event carried by telemetry sinks.
type Event struct {
	Name       string
	Severity   Severity
	Time       time.Time
	Attributes map[string]any
}

// Emitter emits Acorn domain events. Emission is best-effort and must not
// affect the business operation that produced the event.
type Emitter interface {
	Emit(ctx context.Context, event Event)
}
