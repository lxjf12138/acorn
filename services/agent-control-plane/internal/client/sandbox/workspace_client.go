package sandbox

import (
	"context"
	"fmt"

	commonv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/common/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type WorkspaceHostClient interface {
	CreateHostedWorkspace(ctx context.Context, sessionID string, ownerUserID string, sandboxProfileID string, displayName string) (*workspacev1.HostedWorkspace, error)
	GetHostedWorkspace(ctx context.Context, serviceWorkspaceID string) (*workspacev1.HostedWorkspace, error)
	Close() error
}

type GRPCWorkspaceHostClient struct {
	serviceID string
	conn      *grpc.ClientConn
	client    workspacev1.WorkspaceHostServiceClient
}

func NewGRPCWorkspaceHostClient(serviceID string, addr string) (*GRPCWorkspaceHostClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create sandbox workspace client: %w", err)
	}
	return &GRPCWorkspaceHostClient{
		serviceID: serviceID,
		conn:      conn,
		client:    workspacev1.NewWorkspaceHostServiceClient(conn),
	}, nil
}

func (c *GRPCWorkspaceHostClient) CreateHostedWorkspace(ctx context.Context, sessionID string, ownerUserID string, sandboxProfileID string, displayName string) (*workspacev1.HostedWorkspace, error) {
	resp, err := c.client.CreateHostedWorkspace(ctx, &workspacev1.CreateHostedWorkspaceRequest{
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

func (c *GRPCWorkspaceHostClient) GetHostedWorkspace(ctx context.Context, serviceWorkspaceID string) (*workspacev1.HostedWorkspace, error) {
	resp, err := c.client.GetHostedWorkspace(ctx, &workspacev1.GetHostedWorkspaceRequest{
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

func (c *GRPCWorkspaceHostClient) Close() error {
	return c.conn.Close()
}
