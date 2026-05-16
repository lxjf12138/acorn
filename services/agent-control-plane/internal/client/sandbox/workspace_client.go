package sandbox

import (
	"context"
	"fmt"

	commonv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/common/v1"
	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type WorkspaceHostClient interface {
	CreateHostedWorkspace(ctx context.Context, sessionID string, ownerUserID string, sandboxProfileID string, displayName string) (*sandboxv1.HostedWorkspace, error)
	GetHostedWorkspace(ctx context.Context, serviceWorkspaceID string) (*sandboxv1.HostedWorkspace, error)
	GetHostedWorkspaceState(ctx context.Context, sessionID string, ownerUserID string, serviceWorkspaceID string) (*sandboxv1.HostedWorkspaceState, error)
	Close() error
}

type GRPCWorkspaceHostClient struct {
	serviceID string
	conn      *grpc.ClientConn
	client    sandboxv1.WorkspaceHostServiceClient
}

func NewGRPCWorkspaceHostClient(serviceID string, addr string) (*GRPCWorkspaceHostClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create sandbox workspace client: %w", err)
	}
	return &GRPCWorkspaceHostClient{
		serviceID: serviceID,
		conn:      conn,
		client:    sandboxv1.NewWorkspaceHostServiceClient(conn),
	}, nil
}

func (c *GRPCWorkspaceHostClient) CreateHostedWorkspace(ctx context.Context, sessionID string, ownerUserID string, sandboxProfileID string, displayName string) (*sandboxv1.HostedWorkspace, error) {
	resp, err := c.client.CreateHostedWorkspace(ctx, &sandboxv1.CreateHostedWorkspaceRequest{
		Scope: &commonv1.Scope{
			ServiceId: c.serviceID,
			SessionId: sessionID,
			UserId:    ownerUserID,
		},
		SandboxProfileId: sandboxProfileID,
		DisplayName:      displayName,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetWorkspace(), nil
}

func (c *GRPCWorkspaceHostClient) GetHostedWorkspace(ctx context.Context, serviceWorkspaceID string) (*sandboxv1.HostedWorkspace, error) {
	resp, err := c.client.GetHostedWorkspace(ctx, &sandboxv1.GetHostedWorkspaceRequest{
		Scope: &commonv1.Scope{
			ServiceId: c.serviceID,
		},
		ServiceWorkspaceId: serviceWorkspaceID,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetWorkspace(), nil
}

func (c *GRPCWorkspaceHostClient) GetHostedWorkspaceState(ctx context.Context, sessionID string, ownerUserID string, serviceWorkspaceID string) (*sandboxv1.HostedWorkspaceState, error) {
	resp, err := c.client.GetHostedWorkspaceState(ctx, &sandboxv1.GetHostedWorkspaceStateRequest{
		Scope: &commonv1.Scope{
			ServiceId: c.serviceID,
			SessionId: sessionID,
			UserId:    ownerUserID,
		},
		ServiceWorkspaceId: serviceWorkspaceID,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetState(), nil
}

func (c *GRPCWorkspaceHostClient) Close() error {
	return c.conn.Close()
}
