package service

import (
	"context"

	sandboxclient "github.com/lxjf12138/acorn/services/agent-control-plane/internal/client/sandbox"
)

type SandboxResourceAuthority struct {
	client sandboxclient.ResourceContentClient
}

func NewSandboxResourceAuthority(client sandboxclient.ResourceContentClient) *SandboxResourceAuthority {
	return &SandboxResourceAuthority{client: client}
}

func (a *SandboxResourceAuthority) OpenResource(ctx context.Context, resourceID string) (ResourceChunkStream, error) {
	return a.client.OpenResource(ctx, resourceID)
}
