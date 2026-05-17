package observability

import (
	"context"
	"testing"

	"github.com/lxjf12138/acorn/packages/servicekit"
)

func TestInitDisabledReturnsNoop(t *testing.T) {
	providers, err := Init(context.Background(), testBuildInfo(), Config{})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if providers.TracingEnabled {
		t.Fatal("expected tracing disabled")
	}
	if err := providers.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
}

func TestInitStdoutTracing(t *testing.T) {
	providers, err := Init(context.Background(), testBuildInfo(), Config{
		Enabled: true,
		Tracing: TracingConfig{
			Enabled:     true,
			Exporter:    ExporterStdout,
			SampleRatio: 2,
		},
	})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if !providers.TracingEnabled {
		t.Fatal("expected tracing enabled")
	}
	if err := providers.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
}

func TestInitUnknownExporter(t *testing.T) {
	_, err := Init(context.Background(), testBuildInfo(), Config{
		Enabled: true,
		Tracing: TracingConfig{
			Enabled:  true,
			Exporter: "wat",
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInitOTLPRequiresEndpoint(t *testing.T) {
	_, err := Init(context.Background(), testBuildInfo(), Config{
		Enabled: true,
		Tracing: TracingConfig{
			Enabled:  true,
			Exporter: ExporterOTLP,
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func testBuildInfo() servicekit.BuildInfo {
	return servicekit.BuildInfo{ID: "svc-id", Name: "svc-name", Version: "test"}
}
