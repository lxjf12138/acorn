package sandbox

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/middleware"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	commonv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/common/v1"
	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	"google.golang.org/grpc"
)

type ResourceContentClient interface {
	OpenResource(ctx context.Context, resourceID string) (resourcev1.ResourceContentService_OpenResourceClient, error)
	Close() error
}

type GRPCResourceContentClient struct {
	serviceID string
	conn      *grpc.ClientConn
	client    resourcev1.ResourceContentServiceClient
}

func NewGRPCResourceContentClient(serviceID string, addr string, middlewares []middleware.Middleware) (*GRPCResourceContentClient, error) {
	opts := []kgrpc.ClientOption{kgrpc.WithEndpoint(addr)}
	if len(middlewares) > 0 {
		opts = append(opts, kgrpc.WithMiddleware(middlewares...), kgrpc.WithStreamMiddleware(middlewares...))
	}
	conn, err := kgrpc.DialInsecure(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("create sandbox resource content client: %w", err)
	}
	return &GRPCResourceContentClient{
		serviceID: serviceID,
		conn:      conn,
		client:    resourcev1.NewResourceContentServiceClient(conn),
	}, nil
}

func (c *GRPCResourceContentClient) OpenResource(ctx context.Context, resourceID string) (resourcev1.ResourceContentService_OpenResourceClient, error) {
	return c.client.OpenResource(ctx, &resourcev1.OpenResourceRequest{
		Scope: &commonv1.Scope{
			ServiceId: c.serviceID,
		},
		ResourceId: resourceID,
	})
}

func (c *GRPCResourceContentClient) Close() error {
	return c.conn.Close()
}
