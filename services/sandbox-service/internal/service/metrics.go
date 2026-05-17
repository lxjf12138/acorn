package service

import (
	"context"
	"sync"

	"github.com/lxjf12138/acorn/packages/core/telemetry"
	leasedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspacelease"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	serviceMetricsOnce sync.Once
	serviceMetricsInst *serviceMetrics
)

type serviceMetrics struct {
	workspaceLeaseBusyTotal metric.Int64Counter
}

func metrics() *serviceMetrics {
	serviceMetricsOnce.Do(func() {
		meter := otel.Meter("sandbox-service/service")
		workspaceLeaseBusyTotal, _ := meter.Int64Counter(telemetry.MetricWorkspaceLeaseBusyTotal)
		serviceMetricsInst = &serviceMetrics{workspaceLeaseBusyTotal: workspaceLeaseBusyTotal}
	})
	return serviceMetricsInst
}

func recordWorkspaceLeaseBusy(ctx context.Context, mode leasedomain.Mode, reason string) {
	metrics().workspaceLeaseBusyTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String(telemetry.AttrLeaseMode, string(mode)),
		attribute.String(telemetry.AttrLeaseReason, reason),
		attribute.String(telemetry.AttrStatus, telemetry.StatusBusy),
	))
}
