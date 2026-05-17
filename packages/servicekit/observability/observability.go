package observability

import (
	"context"
	"errors"
	"fmt"

	"github.com/lxjf12138/acorn/packages/servicekit"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace/noop"
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
	if !cfg.Enabled || !cfg.Tracing.Enabled || cfg.Tracing.Exporter == ExporterNoop {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return noopProviders(), nil
	}

	exporter, err := traceExporter(ctx, cfg.Tracing)
	if err != nil {
		return nil, err
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sampler(cfg.Tracing.SampleRatio)),
		sdktrace.WithResource(traceResource(info, cfg)),
	)
	otel.SetTracerProvider(provider)
	return &Providers{
		TracingEnabled: true,
		MetricsEnabled: false,
		Shutdown:       provider.Shutdown,
	}, nil
}

func noopProviders() *Providers {
	return &Providers{
		Shutdown: func(context.Context) error { return nil },
	}
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

func traceResource(info servicekit.BuildInfo, cfg Config) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(info.Name),
		semconv.ServiceVersion(info.Version),
		attribute.String("deployment.environment", cfg.Environment),
		attribute.String("acorn.service.id", info.ID),
	)
}
