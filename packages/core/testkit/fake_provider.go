package testkit

import (
	"context"
	"encoding/json"
	"sync"

	toolv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/tool/v1"
	providercore "github.com/lxjf12138/acorn/packages/core/provider"
	toolcore "github.com/lxjf12138/acorn/packages/core/tool"
	"google.golang.org/protobuf/proto"
)

type ToolHandler func(ctx context.Context, req *toolv1.ToolCallRequest) (*toolv1.ToolCallResult, error)

type FakeProvider struct {
	mu        sync.RWMutex
	id        string
	kind      string
	toolSpecs []*toolv1.ToolSpec
	handlers  map[string]ToolHandler
}

func NewFakeProvider(providerID string, kind string, tools []*toolv1.ToolSpec) *FakeProvider {
	fp := &FakeProvider{
		id:        providerID,
		kind:      kind,
		toolSpecs: cloneToolSpecs(tools),
		handlers:  make(map[string]ToolHandler),
	}
	fp.RegisterToolHandler("fake.echo", echoToolHandler)
	return fp
}

func NewFakeProviderWithEcho(providerID string) *FakeProvider {
	return NewFakeProvider(providerID, "fake", []*toolv1.ToolSpec{
		{
			Name:             "fake.echo",
			Description:      "Echo back input text",
			InputSchemaJson:  []byte(`{"type":"object","properties":{"text":{"type":"string"}}}`),
			Risk:             "low",
			SideEffect:       "none",
			RequiresApproval: false,
		},
	})
}

func (f *FakeProvider) ID() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.id
}

func (f *FakeProvider) Type() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.kind
}

func (f *FakeProvider) ListTools(context.Context) ([]*toolv1.ToolSpec, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return cloneToolSpecs(f.toolSpecs), nil
}

func (f *FakeProvider) CallTool(ctx context.Context, req *toolv1.ToolCallRequest) (*toolv1.ToolCallResult, error) {
	f.mu.RLock()
	handler, ok := f.handlers[req.GetToolName()]
	f.mu.RUnlock()
	if !ok {
		return nil, toolcore.ErrToolNotFound
	}
	return handler(ctx, proto.Clone(req).(*toolv1.ToolCallRequest))
}

func (f *FakeProvider) RegisterToolHandler(name string, handler ToolHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers[name] = handler
}

var _ providercore.Provider = (*FakeProvider)(nil)

func cloneToolSpecs(tools []*toolv1.ToolSpec) []*toolv1.ToolSpec {
	out := make([]*toolv1.ToolSpec, 0, len(tools))
	for _, tool := range tools {
		out = append(out, proto.Clone(tool).(*toolv1.ToolSpec))
	}
	return out
}

func echoToolHandler(_ context.Context, req *toolv1.ToolCallRequest) (*toolv1.ToolCallResult, error) {
	type echoArgs struct {
		Text string `json:"text"`
	}
	var args echoArgs
	_ = json.Unmarshal(req.GetArgumentsJson(), &args)
	output, _ := json.Marshal(map[string]string{"text": args.Text})
	return &toolv1.ToolCallResult{
		Content: []*toolv1.ToolContent{
			{
				Type: "text",
				Text: args.Text,
				Name: "echo",
			},
		},
		StructuredOutputJson: output,
	}, nil
}
