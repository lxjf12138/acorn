package testkit

import (
	"context"
	"encoding/json"
	"sync"

	toolv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/tool/v1"
	toolcore "github.com/lxjf12138/acorn/packages/core/tool"
	toolprovidercore "github.com/lxjf12138/acorn/packages/core/toolprovider"
	"google.golang.org/protobuf/proto"
)

type ToolHandler func(ctx context.Context, req *toolv1.ToolCallRequest) (*toolv1.ToolCallResult, error)

type FakeToolProvider struct {
	mu        sync.RWMutex
	id        string
	kind      string
	toolSpecs []*toolv1.ToolSpec
	handlers  map[string]ToolHandler
}

func NewFakeToolProvider(toolProviderID string, kind string, tools []*toolv1.ToolSpec) *FakeToolProvider {
	fp := &FakeToolProvider{
		id:        toolProviderID,
		kind:      kind,
		toolSpecs: cloneToolSpecs(tools),
		handlers:  make(map[string]ToolHandler),
	}
	fp.RegisterToolHandler("fake.echo", echoToolHandler)
	return fp
}

func NewFakeToolProviderWithEcho(toolProviderID string) *FakeToolProvider {
	return NewFakeToolProvider(toolProviderID, "fake", []*toolv1.ToolSpec{
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

func (f *FakeToolProvider) ID() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.id
}

func (f *FakeToolProvider) Type() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.kind
}

func (f *FakeToolProvider) ListTools(context.Context) ([]*toolv1.ToolSpec, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return cloneToolSpecs(f.toolSpecs), nil
}

func (f *FakeToolProvider) CallTool(ctx context.Context, req *toolv1.ToolCallRequest) (*toolv1.ToolCallResult, error) {
	f.mu.RLock()
	handler, ok := f.handlers[req.GetToolName()]
	f.mu.RUnlock()
	if !ok {
		return nil, toolcore.ErrToolNotFound
	}
	return handler(ctx, proto.Clone(req).(*toolv1.ToolCallRequest))
}

func (f *FakeToolProvider) RegisterToolHandler(name string, handler ToolHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers[name] = handler
}

var _ toolprovidercore.ToolProvider = (*FakeToolProvider)(nil)

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
