package server

import (
	"context"
	"sync"

	"github.com/lxjf12138/acorn/packages/core/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	httpMetricsOnce sync.Once
	httpMetricsInst *httpMetrics
)

type httpMetrics struct {
	resourceTransferBytes metric.Int64Counter
	resourceTransferTotal metric.Int64Counter
}

func httpMetricsForServer() *httpMetrics {
	httpMetricsOnce.Do(func() {
		meter := otel.Meter("agent-control-plane/server")
		resourceTransferBytes, _ := meter.Int64Counter(telemetry.MetricResourceTransferBytesTotal)
		resourceTransferTotal, _ := meter.Int64Counter(telemetry.MetricResourceTransferTotal)
		httpMetricsInst = &httpMetrics{
			resourceTransferBytes: resourceTransferBytes,
			resourceTransferTotal: resourceTransferTotal,
		}
	})
	return httpMetricsInst
}

func recordDownloadTransfer(ctx context.Context, status string, authorityServiceID string, sizeBytes int64) {
	inst := httpMetricsForServer()
	attrs := []attribute.KeyValue{
		attribute.String(telemetry.AttrOperation, "download"),
		attribute.String(telemetry.AttrStatus, status),
	}
	if authorityServiceID != "" {
		attrs = append(attrs, attribute.String(telemetry.AttrResourceAuthorityServiceID, authorityServiceID))
	}
	options := metric.WithAttributes(attrs...)
	inst.resourceTransferTotal.Add(ctx, 1, options)
	if status == telemetry.StatusOK && sizeBytes > 0 {
		inst.resourceTransferBytes.Add(ctx, sizeBytes, options)
	}
}
