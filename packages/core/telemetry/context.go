package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

func TraceContext(ctx context.Context) (traceID string, spanID string) {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return "", ""
	}
	return spanContext.TraceID().String(), spanContext.SpanID().String()
}
