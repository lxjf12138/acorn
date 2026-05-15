package server

import (
	klog "github.com/go-kratos/kratos/v2/log"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/lxjf12138/acorn/packages/servicekit"

	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/conf"
)

func NewGRPCServer(cfg *conf.Config, logger klog.Logger) *kgrpc.Server {
	return kgrpc.NewServer(
		kgrpc.Address(cfg.Server.GRPC.Addr),
		kgrpc.Timeout(cfg.Server.GRPC.TimeoutDuration()),
		kgrpc.Middleware(servicekit.DefaultServerMiddleware(logger)...),
	)
}
