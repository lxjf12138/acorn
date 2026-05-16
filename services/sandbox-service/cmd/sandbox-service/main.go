package main

import (
	"flag"

	klog "github.com/go-kratos/kratos/v2/log"
	"github.com/lxjf12138/acorn/packages/servicekit"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/app"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/conf"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/descriptor"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/server"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/service"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/version"
)

func main() {
	confPath := flag.String("conf", "configs/config.yaml", "config path")
	flag.Parse()

	cfg, err := conf.Load(*confPath)
	if err != nil {
		panic(err)
	}

	version.ServiceName = cfg.Service.Name
	if version.Version == "" || version.Version == "dev" {
		version.Version = cfg.Service.Version
	}

	logger := servicekit.NewLogger(servicekit.BuildInfo{
		Name:    cfg.Service.Name,
		Version: version.Version,
	}, cfg.Log.Level)
	helper := klog.NewHelper(logger)
	statusService := service.NewStatusService()
	descriptorSource := descriptor.NewSourceFromConfig(cfg, version.Version)
	descriptorService := service.NewDescriptorService(descriptorSource)
	workspaceStore := workspacedomain.NewMemoryStore()
	workspaceService := service.NewWorkspaceService(cfg.Service.ID, descriptorSource, workspaceStore)

	httpSrv := server.NewHTTPServer(cfg, statusService, descriptorSource, logger)
	grpcSrv := server.NewGRPCServer(cfg, descriptorService, workspaceService, logger)

	kratosApp := app.New(cfg.Service.Name, version.Version, logger, httpSrv, grpcSrv)

	helper.Infow(
		"msg", "starting service",
		"service.id", cfg.Service.ID,
		"service.name", cfg.Service.Name,
		"http.addr", cfg.Server.HTTP.Addr,
		"grpc.addr", cfg.Server.GRPC.Addr,
	)

	if err := kratosApp.Run(); err != nil {
		helper.Errorw("msg", "service terminated with error", "err", err)
		panic(err)
	}
}
