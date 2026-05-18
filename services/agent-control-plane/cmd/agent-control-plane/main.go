package main

import (
	"context"
	"flag"

	klog "github.com/go-kratos/kratos/v2/log"
	"github.com/lxjf12138/acorn/packages/servicekit"
	"github.com/lxjf12138/acorn/packages/servicekit/localblob"
	"github.com/lxjf12138/acorn/packages/servicekit/observability"

	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/app"
	sandboxclient "github.com/lxjf12138/acorn/services/agent-control-plane/internal/client/sandbox"
	"github.com/lxjf12138/acorn/services/agent-control-plane/internal/conf"
	resourcedomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/resource"
	sandboxpolicydomain "github.com/lxjf12138/acorn/services/agent-control-plane/internal/domain/sandboxpolicy"
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

	buildInfo := servicekit.BuildInfo{
		ID:      cfg.Service.ID,
		Name:    cfg.Service.Name,
		Version: version.Version,
	}
	logger := servicekit.NewLogger(buildInfo, cfg.Log.Level)
	obs, err := observability.Init(context.Background(), buildInfo, cfg.Observability)
	if err != nil {
		panic(err)
	}
	defer obs.Shutdown(context.Background())
	clientMiddleware := servicekit.DefaultClientMiddleware(servicekit.ClientMiddlewareOptions{
		TracingEnabled: obs.TracingEnabled,
		MetricsEnabled: obs.MetricsEnabled,
	})
	helper := klog.NewHelper(logger)
	statusService := service.NewStatusService()
	resourceStore := resourcedomain.NewMemoryStore()
	resourceService := service.NewResourceService(resourceStore)
	blobStore, err := localblob.NewStore(localblob.Config{BaseDir: cfg.Resource.BlobRoot})
	if err != nil {
		panic(err)
	}
	workspaceStore := workspacedomain.NewMemoryStore()
	workspaceClient, err := sandboxclient.NewGRPCWorkspaceHostClient(cfg.Sandbox.ServiceID, cfg.Sandbox.GRPCAddr, clientMiddleware)
	if err != nil {
		panic(err)
	}
	defer workspaceClient.Close()
	resourceContentClient, err := sandboxclient.NewGRPCResourceContentClient(cfg.Sandbox.ServiceID, cfg.Sandbox.GRPCAddr, clientMiddleware)
	if err != nil {
		panic(err)
	}
	defer resourceContentClient.Close()
	resourceGatewayService := service.NewResourceGatewayService(resourceStore, map[string]service.ResourceAuthorityClient{
		cfg.Service.ID:        service.NewLocalResourceAuthority(cfg.Service.ID, blobStore),
		cfg.Sandbox.ServiceID: service.NewSandboxResourceAuthority(resourceContentClient),
	})
	uploadService := service.NewUploadService(cfg.Service.ID, blobStore, resourceService, cfg.Resource.UploadMaxBytes)
	sandboxPolicyResolver := sandboxpolicydomain.NewConfigResolver(cfg.SandboxPolicies, cfg.Sandbox.DefaultProfileID)
	workspaceService := service.NewWorkspaceServiceWithResourcesGatewayAndPolicy(workspaceStore, workspaceClient, resourceService, resourceGatewayService, cfg.Sandbox.ServiceID, sandboxPolicyResolver)

	httpSrv := server.NewHTTPServer(cfg, statusService, workspaceService, resourceService, resourceGatewayService, uploadService, logger, obs.TracingEnabled)
	grpcSrv := server.NewGRPCServer(cfg, resourceService, logger, obs.TracingEnabled)

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
