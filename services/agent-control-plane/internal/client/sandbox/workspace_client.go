package sandbox

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/go-kratos/kratos/v2/middleware"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
	commonv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/common/v1"
	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
)

type WorkspaceHostClient interface {
	GetCapabilityDescriptor(ctx context.Context) (*capabilityv1.CapabilityDescriptor, error)
	CreateHostedWorkspace(ctx context.Context, sessionID string, ownerUserID string, sandboxProfileID string, displayName string) (*sandboxv1.HostedWorkspace, error)
	GetHostedWorkspace(ctx context.Context, serviceWorkspaceID string) (*sandboxv1.HostedWorkspace, error)
	GetHostedWorkspaceState(ctx context.Context, sessionID string, ownerUserID string, serviceWorkspaceID string) (*sandboxv1.HostedWorkspaceState, error)
	ListWorkspaceDir(ctx context.Context, input ListWorkspaceDirInput) (*sandboxv1.ListWorkspaceDirResponse, error)
	PreviewWorkspaceFile(ctx context.Context, input PreviewWorkspaceFileInput) (*sandboxv1.PreviewWorkspaceFileResponse, error)
	ExportWorkspacePath(ctx context.Context, input ExportWorkspacePathInput) (*sandboxv1.ExportWorkspacePathResponse, error)
	ImportResourceToWorkspace(ctx context.Context, input ImportResourceInput, reader io.Reader) (*sandboxv1.ImportResourceToWorkspaceResponse, error)
	ExecWorkspaceCommand(ctx context.Context, input ExecWorkspaceCommandInput) (*sandboxv1.ExecWorkspaceCommandResponse, error)
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

type ExecWorkspaceCommandInput struct {
	SessionID          string
	UserID             string
	ServiceWorkspaceID string

	Command string
	Args    []string
	CWD     string
	Env     map[string]string

	Timeout        time.Duration
	MaxStdoutBytes int64
	MaxStderrBytes int64
}

type GRPCWorkspaceHostClient struct {
	serviceID  string
	conn       *grpc.ClientConn
	descriptor capabilityv1.CapabilityDescriptorServiceClient
	client     sandboxv1.WorkspaceHostServiceClient
	view       sandboxv1.WorkspaceViewServiceClient
	transfer   sandboxv1.WorkspaceTransferServiceClient
	exec       sandboxv1.WorkspaceExecServiceClient
}

func NewGRPCWorkspaceHostClient(serviceID string, addr string, middlewares []middleware.Middleware) (*GRPCWorkspaceHostClient, error) {
	opts := []kgrpc.ClientOption{kgrpc.WithEndpoint(addr)}
	if len(middlewares) > 0 {
		opts = append(opts, kgrpc.WithMiddleware(middlewares...), kgrpc.WithStreamMiddleware(middlewares...))
	}
	conn, err := kgrpc.DialInsecure(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("create sandbox workspace client: %w", err)
	}
	return &GRPCWorkspaceHostClient{
		serviceID:  serviceID,
		conn:       conn,
		descriptor: capabilityv1.NewCapabilityDescriptorServiceClient(conn),
		client:     sandboxv1.NewWorkspaceHostServiceClient(conn),
		view:       sandboxv1.NewWorkspaceViewServiceClient(conn),
		transfer:   sandboxv1.NewWorkspaceTransferServiceClient(conn),
		exec:       sandboxv1.NewWorkspaceExecServiceClient(conn),
	}, nil
}

func (c *GRPCWorkspaceHostClient) GetCapabilityDescriptor(ctx context.Context) (*capabilityv1.CapabilityDescriptor, error) {
	resp, err := c.descriptor.GetCapabilityDescriptor(ctx, &capabilityv1.GetCapabilityDescriptorRequest{})
	if err != nil {
		return nil, err
	}
	return resp.GetCapabilityDescriptor(), nil
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

func (c *GRPCWorkspaceHostClient) ExecWorkspaceCommand(ctx context.Context, input ExecWorkspaceCommandInput) (*sandboxv1.ExecWorkspaceCommandResponse, error) {
	return c.exec.ExecWorkspaceCommand(ctx, &sandboxv1.ExecWorkspaceCommandRequest{
		Scope: &commonv1.Scope{
			ServiceId: c.serviceID,
			SessionId: input.SessionID,
			UserId:    input.UserID,
		},
		ServiceWorkspaceId: input.ServiceWorkspaceID,
		Command:            input.Command,
		Args:               append([]string(nil), input.Args...),
		Cwd:                input.CWD,
		Env:                cloneStringMap(input.Env),
		Timeout:            durationProto(input.Timeout),
		MaxStdoutBytes:     input.MaxStdoutBytes,
		MaxStderrBytes:     input.MaxStderrBytes,
	})
}

func (c *GRPCWorkspaceHostClient) Close() error {
	return c.conn.Close()
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func durationProto(value time.Duration) *durationpb.Duration {
	if value <= 0 {
		return nil
	}
	return durationpb.New(value)
}
