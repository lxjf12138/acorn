package sandbox

import (
	"context"
	"fmt"

	commonv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/common/v1"
	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

func NewGRPCResourceContentClient(serviceID string, addr string) (*GRPCResourceContentClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
