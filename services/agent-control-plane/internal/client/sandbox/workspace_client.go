package sandbox

import (
	"context"
	"fmt"
	"io"

	commonv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/common/v1"
	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
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
	ExportWorkspacePath(ctx context.Context, input ExportWorkspacePathInput) (*sandboxv1.ExportWorkspacePathResponse, error)
	ImportResourceToWorkspace(ctx context.Context, input ImportResourceInput, reader io.Reader) (*sandboxv1.ImportResourceToWorkspaceResponse, error)
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

type ExportWorkspacePathInput struct {
	SessionID          string
	UserID             string
	ServiceWorkspaceID string
	Path               string
	ResourceName       string
	MimeType           string
}

type ImportResourceInput struct {
	SessionID          string
	UserID             string
	ServiceWorkspaceID string
	Resource           *resourcev1.ResourceRef
	DestinationPath    string
	ConflictPolicy     sandboxv1.ImportConflictPolicy
}

type GRPCWorkspaceHostClient struct {
	serviceID string
	conn      *grpc.ClientConn
	client    sandboxv1.WorkspaceHostServiceClient
	view      sandboxv1.WorkspaceViewServiceClient
	transfer  sandboxv1.WorkspaceTransferServiceClient
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
		transfer:  sandboxv1.NewWorkspaceTransferServiceClient(conn),
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

func (c *GRPCWorkspaceHostClient) ExportWorkspacePath(ctx context.Context, input ExportWorkspacePathInput) (*sandboxv1.ExportWorkspacePathResponse, error) {
	return c.transfer.ExportWorkspacePath(ctx, &sandboxv1.ExportWorkspacePathRequest{
		Scope: &commonv1.Scope{
			ServiceId: c.serviceID,
			SessionId: input.SessionID,
			UserId:    input.UserID,
		},
		ServiceWorkspaceId: input.ServiceWorkspaceID,
		Path:               input.Path,
		ResourceName:       input.ResourceName,
		MimeType:           input.MimeType,
	})
}

func (c *GRPCWorkspaceHostClient) ImportResourceToWorkspace(ctx context.Context, input ImportResourceInput, reader io.Reader) (*sandboxv1.ImportResourceToWorkspaceResponse, error) {
	stream, err := c.transfer.ImportResourceToWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	if err := stream.Send(&sandboxv1.ImportResourceToWorkspaceRequest{
		Payload: &sandboxv1.ImportResourceToWorkspaceRequest_Header{
			Header: &sandboxv1.ImportResourceToWorkspaceHeader{
				Scope: &commonv1.Scope{
					ServiceId: c.serviceID,
					SessionId: input.SessionID,
					UserId:    input.UserID,
				},
				ServiceWorkspaceId: input.ServiceWorkspaceID,
				Resource:           input.Resource,
				DestinationPath:    input.DestinationPath,
				ConflictPolicy:     input.ConflictPolicy,
			},
		},
	}); err != nil {
		return nil, err
	}
	buf := make([]byte, 64*1024)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			if err := stream.Send(&sandboxv1.ImportResourceToWorkspaceRequest{
				Payload: &sandboxv1.ImportResourceToWorkspaceRequest_Data{Data: append([]byte(nil), buf[:n]...)},
			}); err != nil {
				return nil, err
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return stream.CloseAndRecv()
			}
			_ = stream.CloseSend()
			return nil, readErr
		}
	}
}

func (c *GRPCWorkspaceHostClient) Close() error {
	return c.conn.Close()
}
