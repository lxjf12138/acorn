package server

import (
	klog "github.com/go-kratos/kratos/v2/log"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	sandboxv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/sandbox/v1"
	"github.com/lxjf12138/acorn/packages/servicekit"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/conf"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/service"
)

func NewGRPCServer(cfg *conf.Config, descriptorService *service.DescriptorService, workspaceService *service.WorkspaceService, viewService *service.WorkspaceViewService, transferService *service.WorkspaceTransferService, execService *service.WorkspaceExecService, resourceContentService *service.ResourceContentService, logger klog.Logger, tracingEnabled bool) *kgrpc.Server {
	srv := kgrpc.NewServer(
		kgrpc.Address(cfg.Server.GRPC.Addr),
		kgrpc.Timeout(cfg.Server.GRPC.TimeoutDuration()),
		kgrpc.Middleware(servicekit.DefaultServerMiddleware(servicekit.ServerMiddlewareOptions{
			Logger:         logger,
			TracingEnabled: tracingEnabled,
		})...),
	)
	capabilityv1.RegisterCapabilityDescriptorServiceServer(srv, descriptorService)
	sandboxv1.RegisterWorkspaceHostServiceServer(srv, workspaceService)
	sandboxv1.RegisterWorkspaceViewServiceServer(srv, viewService)
	sandboxv1.RegisterWorkspaceTransferServiceServer(srv, transferService)
	if execService != nil {
		sandboxv1.RegisterWorkspaceExecServiceServer(srv, execService)
	}
	resourcev1.RegisterResourceContentServiceServer(srv, resourceContentService)
	return srv
}
