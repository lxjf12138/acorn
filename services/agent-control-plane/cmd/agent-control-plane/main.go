package main

import (
	"flag"

	klog "github.com/go-kratos/kratos/v2/log"
	"github.com/lxjf12138/acorn/packages/servicekit"

	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/app"
	sandboxclient "github.com/lxjf12138/acorn/services/agent-control-plane/internal/client/sandbox"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/conf"
	resourcedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/resource"
	workspacedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/workspace"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/server"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/service"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/version"
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
	resourceStore := resourcedomain.NewMemoryStore()
	resourceService := service.NewResourceService(resourceStore)
	workspaceStore := workspacedomain.NewMemoryStore()
	workspaceClient, err := sandboxclient.NewGRPCWorkspaceHostClient(cfg.Sandbox.ServiceID, cfg.Sandbox.GRPCAddr)
	if err != nil {
		panic(err)
	}
	defer workspaceClient.Close()
	workspaceService := service.NewWorkspaceService(workspaceStore, workspaceClient, cfg.Sandbox.ServiceID, cfg.Sandbox.DefaultProfileID)

	httpSrv := server.NewHTTPServer(cfg, statusService, workspaceService, resourceService, logger)
	grpcSrv := server.NewGRPCServer(cfg, resourceService, logger)

	kratosApp := app.New(cfg.Service.Name, version.Version, logger, httpSrv, grpcSrv)

	helper.Infow(
		"msg", "starting service",
		"service", cfg.Service.Name,
		"http.addr", cfg.Server.HTTP.Addr,
		"grpc.addr", cfg.Server.GRPC.Addr,
	)

	if err := kratosApp.Run(); err != nil {
		helper.Errorw("msg", "service terminated with error", "err", err)
		panic(err)
	}
}
