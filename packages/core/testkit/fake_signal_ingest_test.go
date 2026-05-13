package testkit

import (
	"context"
	"testing"

	signalv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/signal/v1"
)

func TestFakeSignalIngestRecordsSignals(t *testing.T) {
	ingest := NewFakeSignalIngest()
	_ = ingest.Emit(context.Background(), &signalv1.Signal{Id: "sig-1"})
	_ = ingest.Emit(context.Background(), &signalv1.Signal{Id: "sig-2"})

	signals := ingest.Signals()
	if len(signals) != 2 {
		t.Fatalf("unexpected signal count: %d", len(signals))
	}
	if signals[0].GetId() != "sig-1" || signals[1].GetId() != "sig-2" {
		t.Fatalf("unexpected signal order: %q, %q", signals[0].GetId(), signals[1].GetId())
	}
}
