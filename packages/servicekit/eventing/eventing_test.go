package eventing

import (
	"context"
	"testing"

	"github.com/lxjf12138/acorn/packages/core/events"
)

func TestNoopEmitterDoesNotPanic(t *testing.T) {
	NoopEmitter{}.Emit(context.Background(), events.Event{Name: events.ResourceUploaded})
}

func TestMultiEmitterCallsChildren(t *testing.T) {
	first := &recordingEmitter{}
	second := &recordingEmitter{}
	emitter := NewMultiEmitter(first, nil, second)
	emitter.Emit(context.Background(), events.Event{Name: events.ResourceUploaded})
	if first.count != 1 || second.count != 1 {
		t.Fatalf("expected both emitters called, got first=%d second=%d", first.count, second.count)
	}
}

func TestOTelLogEmitterDoesNotPanic(t *testing.T) {
	emitter := NewOTelLogEmitter("test/events")
	emitter.Emit(context.Background(), events.Event{
		Name:     events.WorkspaceExecCompleted,
		Severity: events.SeverityInfo,
		Attributes: map[string]any{
			events.AttrExecCommandName: "go",
			events.AttrExecArgCount:    1,
		},
	})
}

func TestStructuredLoggerEmitterDoesNotPanic(t *testing.T) {
	emitter := NewStructuredLoggerEmitter(nil)
	emitter.Emit(context.Background(), events.Event{Name: events.ResourceUploaded})
}

type recordingEmitter struct {
	count int
}

func (e *recordingEmitter) Emit(context.Context, events.Event) {
	e.count++
}
