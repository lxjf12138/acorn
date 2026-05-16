package testkit

import (
	"context"
	"testing"

	toolv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/tool/v1"
)

func TestFakeToolProviderEcho(t *testing.T) {
	toolProvider := NewFakeToolProviderWithEcho("fake-service")
	result, err := toolProvider.CallTool(context.Background(), &toolv1.ToolCallRequest{
		TargetServiceId: "fake-service",
		ToolName:        "fake.echo",
		ArgumentsJson:   []byte(`{"text":"hello"}`),
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if got := result.GetContent()[0].GetText(); got != "hello" {
		t.Fatalf("unexpected content text: %q", got)
	}
	if got := string(result.GetStructuredOutputJson()); got != `{"text":"hello"}` {
		t.Fatalf("unexpected structured output: %s", got)
	}
}
