package testkit

import (
	"context"
	"testing"

	eventv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/event/v1"
)

func TestFakeEventBusRecordsEvents(t *testing.T) {
	bus := NewFakeEventBus()
	_ = bus.Publish(context.Background(), &eventv1.Event{Id: "evt-1"})

	events := bus.Events()
	if len(events) != 1 {
		t.Fatalf("unexpected event count: %d", len(events))
	}
	if events[0].GetId() != "evt-1" {
		t.Fatalf("unexpected event id: %q", events[0].GetId())
	}
}
