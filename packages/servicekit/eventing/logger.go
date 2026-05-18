package eventing

import (
	"context"
	"time"

	klog "github.com/go-kratos/kratos/v2/log"
	"github.com/lxjf12138/acorn/packages/core/events"
	"go.opentelemetry.io/otel/trace"
)

type StructuredLoggerEmitter struct {
	logger klog.Logger
}

func NewStructuredLoggerEmitter(logger klog.Logger) *StructuredLoggerEmitter {
	return &StructuredLoggerEmitter{logger: logger}
}

func (e *StructuredLoggerEmitter) Emit(ctx context.Context, event events.Event) {
	if e == nil || e.logger == nil || event.Name == "" {
		return
	}
	timestamp := event.Time
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	fields := []any{
		"event.name", event.Name,
		"event.severity", string(event.Severity),
		"event.time", timestamp.Format(time.RFC3339Nano),
	}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		fields = append(fields, "trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String())
	}
	for key, value := range event.Attributes {
		if key == "" || value == nil {
			continue
		}
		fields = append(fields, key, value)
	}
	_ = e.logger.Log(klog.LevelInfo, fields...)
}
