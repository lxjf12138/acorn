package server

import (
	klog "github.com/go-kratos/kratos/v2/log"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	resourcev1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/resource/v1"
	"github.com/lxjf12138/acorn/packages/servicekit"

	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/conf"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/service"
)

func NewGRPCServer(cfg *conf.Config, resourceService *service.ResourceService, logger klog.Logger) *kgrpc.Server {
	srv := kgrpc.NewServer(
		kgrpc.Address(cfg.Server.GRPC.Addr),
		kgrpc.Timeout(cfg.Server.GRPC.TimeoutDuration()),
		kgrpc.Middleware(servicekit.DefaultServerMiddleware(logger)...),
	)
	resourcev1.RegisterResourceServiceServer(srv, resourceService)
	return srv
}
