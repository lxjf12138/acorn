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
	ListWorkspaceDir(ctx context.Context, input ListWorkspaceDirInput) (*sandboxv1.ListWorkspaceDirResponse, error)
	PreviewWorkspaceFile(ctx context.Context, input PreviewWorkspaceFileInput) (*sandboxv1.PreviewWorkspaceFileResponse, error)
	Close() error
}

type ListWorkspaceDirInput struct {
	SessionID          string
	UserID             string
	ServiceWorkspaceID string
	Path               string
	PageSize           int32
	PageToken          string
}

type PreviewWorkspaceFileInput struct {
	SessionID          string
	UserID             string
	ServiceWorkspaceID string
	Path               string
	MaxBytes           int64
}

type GRPCWorkspaceHostClient struct {
	serviceID string
	conn      *grpc.ClientConn
	client    sandboxv1.WorkspaceHostServiceClient
	view      sandboxv1.WorkspaceViewServiceClient
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
		view:      sandboxv1.NewWorkspaceViewServiceClient(conn),
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

func (c *GRPCWorkspaceHostClient) ListWorkspaceDir(ctx context.Context, input ListWorkspaceDirInput) (*sandboxv1.ListWorkspaceDirResponse, error) {
	return c.view.ListWorkspaceDir(ctx, &sandboxv1.ListWorkspaceDirRequest{
		Scope: &commonv1.Scope{
			ServiceId: c.serviceID,
			SessionId: input.SessionID,
			UserId:    input.UserID,
		},
		ServiceWorkspaceId: input.ServiceWorkspaceID,
		Path:               input.Path,
		PageSize:           input.PageSize,
		PageToken:          input.PageToken,
	})
}

func (c *GRPCWorkspaceHostClient) PreviewWorkspaceFile(ctx context.Context, input PreviewWorkspaceFileInput) (*sandboxv1.PreviewWorkspaceFileResponse, error) {
	return c.view.PreviewWorkspaceFile(ctx, &sandboxv1.PreviewWorkspaceFileRequest{
		Scope: &commonv1.Scope{
			ServiceId: c.serviceID,
			SessionId: input.SessionID,
			UserId:    input.UserID,
		},
		ServiceWorkspaceId: input.ServiceWorkspaceID,
		Path:               input.Path,
		MaxBytes:           input.MaxBytes,
	})
}

func (c *GRPCWorkspaceHostClient) Close() error {
	return c.conn.Close()
}
