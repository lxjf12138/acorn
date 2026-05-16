package server

import (
	klog "github.com/go-kratos/kratos/v2/log"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	capabilityv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/capability/v1"
	workspacev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/workspace/v1"
	"github.com/lxjf12138/acorn/packages/servicekit"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/conf"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/service"
)

func NewGRPCServer(cfg *conf.Config, descriptorService *service.DescriptorService, workspaceService *service.WorkspaceService, logger klog.Logger) *kgrpc.Server {
	srv := kgrpc.NewServer(
		kgrpc.Address(cfg.Server.GRPC.Addr),
		kgrpc.Timeout(cfg.Server.GRPC.TimeoutDuration()),
		kgrpc.Middleware(servicekit.DefaultServerMiddleware(logger)...),
	)
	capabilityv1.RegisterCapabilityDescriptorServiceServer(srv, descriptorService)
	workspacev1.RegisterWorkspaceHostServiceServer(srv, workspaceService)
	return srv
}
