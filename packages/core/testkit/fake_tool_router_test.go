package testkit

import (
	"context"
	"testing"

	toolv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/tool/v1"
)

func TestFakeToolRouterRoutesToProvider(t *testing.T) {
	router := NewFakeToolRouter()
	router.AddProvider(NewFakeProviderWithEcho("provider-1"))

	result, err := router.CallTool(context.Background(), &toolv1.ToolCallRequest{
		ProviderId:    "provider-1",
		ToolName:      "fake.echo",
		ArgumentsJson: []byte(`{"text":"hello"}`),
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if got := result.GetContent()[0].GetText(); got != "hello" {
		t.Fatalf("unexpected content text: %q", got)
	}
}
