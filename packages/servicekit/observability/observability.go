package observability

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lxjf12138/acorn/packages/servicekit"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.24.0"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

type Providers struct {
	TracingEnabled bool
	MetricsEnabled bool

	Shutdown func(context.Context) error
}

func Init(ctx context.Context, info servicekit.BuildInfo, cfg Config) (*Providers, error) {
	cfg = cfg.normalized()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	res := observabilityResource(info, cfg)
	tracingShutdown, tracingEnabled, err := initTracing(ctx, res, cfg)
	if err != nil {
		return nil, err
	}
	metricsShutdown, metricsEnabled, err := initMetrics(ctx, res, cfg)
	if err != nil {
		_ = tracingShutdown(ctx)
		return nil, err
	}
	return &Providers{
		TracingEnabled: tracingEnabled,
		MetricsEnabled: metricsEnabled,
		Shutdown: func(ctx context.Context) error {
			return errors.Join(metricsShutdown(ctx), tracingShutdown(ctx))
		},
	}, nil
}

func initTracing(ctx context.Context, res *resource.Resource, cfg Config) (func(context.Context) error, bool, error) {
	if !cfg.Enabled || !cfg.Tracing.Enabled || cfg.Tracing.Exporter == ExporterNoop {
		otel.SetTracerProvider(tracenoop.NewTracerProvider())
		return noopShutdown, false, nil
	}
	exporter, err := traceExporter(ctx, cfg.Tracing)
	if err != nil {
		return nil, false, err
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sampler(cfg.Tracing.SampleRatio)),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	return provider.Shutdown, true, nil
}

func initMetrics(ctx context.Context, res *resource.Resource, cfg Config) (func(context.Context) error, bool, error) {
	if !cfg.Enabled || !cfg.Metrics.Enabled || cfg.Metrics.Exporter == ExporterNoop {
		otel.SetMeterProvider(noop.NewMeterProvider())
		return noopShutdown, false, nil
	}
	reader, err := metricReader(ctx, cfg.Metrics)
	if err != nil {
		return nil, false, err
	}
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithResource(res), sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	return provider.Shutdown, true, nil
}

func noopShutdown(context.Context) error {
	return nil
}

func traceExporter(ctx context.Context, cfg TracingConfig) (sdktrace.SpanExporter, error) {
	switch cfg.Exporter {
	case ExporterStdout:
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	case ExporterOTLP:
		if cfg.OTLPEndpoint == "" {
			return nil, errors.New("observability.tracing.otlp_endpoint is required for otlp exporter")
		}
		return otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint), otlptracegrpc.WithInsecure())
	default:
		return nil, fmt.Errorf("unsupported tracing exporter: %s", cfg.Exporter)
	}
}

func metricReader(ctx context.Context, cfg MetricsConfig) (sdkmetric.Reader, error) {
	switch cfg.Exporter {
	case ExporterStdout:
		exporter, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
		return sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(time.Second)), nil
	case ExporterOTLP:
		if cfg.OTLPEndpoint == "" {
			return nil, errors.New("observability.metrics.otlp_endpoint is required for otlp exporter")
		}
		exporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint), otlpmetricgrpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		return sdkmetric.NewPeriodicReader(exporter), nil
	default:
		return nil, fmt.Errorf("unsupported metrics exporter: %s", cfg.Exporter)
	}
}

func sampler(ratio float64) sdktrace.Sampler {
	switch {
	case ratio <= 0:
		return sdktrace.NeverSample()
	case ratio >= 1:
		return sdktrace.AlwaysSample()
	default:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	}
}

func observabilityResource(info servicekit.BuildInfo, cfg Config) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(info.Name),
		semconv.ServiceVersion(info.Version),
		attribute.String("deployment.environment", cfg.Environment),
		attribute.String("acorn.service.id", info.ID),
	)
}
