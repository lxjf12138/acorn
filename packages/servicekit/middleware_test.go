package servicekit

import "testing"

func TestDefaultServerMiddlewareTracingOption(t *testing.T) {
	withoutTracing := DefaultServerMiddleware(ServerMiddlewareOptions{})
	withTracing := DefaultServerMiddleware(ServerMiddlewareOptions{TracingEnabled: true})
	if len(withTracing) != len(withoutTracing)+1 {
		t.Fatalf("expected tracing middleware to add one entry: without=%d with=%d", len(withoutTracing), len(withTracing))
	}
}

func TestDefaultClientMiddlewareTracingOption(t *testing.T) {
	if got := DefaultClientMiddleware(ClientMiddlewareOptions{}); len(got) != 0 {
		t.Fatalf("expected no client middleware, got %d", len(got))
	}
	if got := DefaultClientMiddleware(ClientMiddlewareOptions{TracingEnabled: true}); len(got) != 1 {
		t.Fatalf("expected tracing client middleware, got %d", len(got))
	}
}
