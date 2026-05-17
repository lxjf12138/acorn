package service

import (
	"context"
	"sync"
	"time"

	"github.com/lxjf12138/acorn/packages/core/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	metricsOnce sync.Once
	metricsInst *serviceMetrics
)

type serviceMetrics struct {
	resourceTransferBytes metric.Int64Counter
	resourceTransferTotal metric.Int64Counter
	workspaceExecTotal    metric.Int64Counter
	workspaceExecDuration metric.Float64Histogram
}

func metrics() *serviceMetrics {
	metricsOnce.Do(func() {
		meter := otel.Meter("agent-control-plane/service")
		resourceTransferBytes, _ := meter.Int64Counter(telemetry.MetricResourceTransferBytesTotal)
		resourceTransferTotal, _ := meter.Int64Counter(telemetry.MetricResourceTransferTotal)
		workspaceExecTotal, _ := meter.Int64Counter(telemetry.MetricWorkspaceExecTotal)
		workspaceExecDuration, _ := meter.Float64Histogram(telemetry.MetricWorkspaceExecDurationSeconds)
		metricsInst = &serviceMetrics{
			resourceTransferBytes: resourceTransferBytes,
			resourceTransferTotal: resourceTransferTotal,
			workspaceExecTotal:    workspaceExecTotal,
			workspaceExecDuration: workspaceExecDuration,
		}
	})
	return metricsInst
}

func recordResourceTransfer(ctx context.Context, operation string, status string, authorityServiceID string, sizeBytes int64) {
	inst := metrics()
	attrs := []attribute.KeyValue{
		attribute.String(telemetry.AttrOperation, operation),
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

func recordWorkspaceExec(ctx context.Context, status string, sandboxProfileID string, duration time.Duration) {
	inst := metrics()
	attrs := []attribute.KeyValue{
		attribute.String(telemetry.AttrStatus, status),
	}
	if sandboxProfileID != "" {
		attrs = append(attrs, attribute.String(telemetry.AttrSandboxProfileID, sandboxProfileID))
	}
	options := metric.WithAttributes(attrs...)
	inst.workspaceExecTotal.Add(ctx, 1, options)
	inst.workspaceExecDuration.Record(ctx, duration.Seconds(), options)
}
