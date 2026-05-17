package main

import (
	"context"
	"flag"
	"time"

	klog "github.com/go-kratos/kratos/v2/log"
	"github.com/lxjf12138/acorn/packages/servicekit"

	"github.com/lxjf12138/acorn/packages/servicekit/localblob"
	"github.com/lxjf12138/acorn/packages/servicekit/observability"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/app"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/conf"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/descriptor"
	exporteddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/exportedresource"
	profiledomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/profile"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/infra/localfs"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/infra/localprocess"
	workspaceleasememory "github.com/lxjf12138/acorn/services/sandbox-service/internal/infra/workspacelease"
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
	helper := klog.NewHelper(logger)
	statusService := service.NewStatusService()
	profileRegistry := profiledomain.NewRegistryFromConfig(cfg.Sandbox)
	descriptorSource := descriptor.NewSourceFromConfig(cfg, version.Version, profileRegistry)
	descriptorService := service.NewDescriptorService(descriptorSource)
	workspaceStore := workspacedomain.NewMemoryStore()
	backingStore, err := localfs.NewWorkspaceStore(localfs.Config{BaseDir: cfg.Sandbox.WorkspaceRoot})
	if err != nil {
		panic(err)
	}
	blobStore, err := localblob.NewStore(localblob.Config{BaseDir: cfg.Sandbox.ResourceBlobRoot})
	if err != nil {
		panic(err)
	}
	exportStore := exporteddomain.NewMemoryStore()
	workspaceLeaseManager := workspaceleasememory.NewMemoryManager()
	workspaceService := service.NewWorkspaceService(cfg.Service.ID, profileRegistry, workspaceStore, backingStore)
	viewService := service.NewWorkspaceViewService(cfg.Service.ID, workspaceStore, backingStore, workspaceLeaseManager)
	transferService := service.NewWorkspaceTransferService(cfg.Service.ID, workspaceStore, backingStore, blobStore, exportStore, workspaceLeaseManager)
	var execService *service.WorkspaceExecService
	if profileRegistry.AnyEnabledHasCapability(profiledomain.CapabilityWorkspaceExec) {
		attachmentService := service.NewWorkspaceAttachmentService(workspaceStore, backingStore)
		localProcessBackend := localprocess.NewBackend(localprocess.Config{
			ID:             "local-process-dev",
			DefaultTimeout: time.Duration(cfg.Sandbox.LocalProcess.DefaultTimeoutSeconds) * time.Second,
			MaxTimeout:     time.Duration(cfg.Sandbox.LocalProcess.MaxTimeoutSeconds) * time.Second,
			MaxStdoutBytes: cfg.Sandbox.LocalProcess.MaxStdoutBytes,
			MaxStderrBytes: cfg.Sandbox.LocalProcess.MaxStderrBytes,
		})
		execService = service.NewWorkspaceExecService(cfg.Service.ID, workspaceStore, profileRegistry, attachmentService, localProcessBackend, workspaceLeaseManager)
	}
	resourceContentService := service.NewResourceContentService(cfg.Service.ID, exportStore, blobStore)

	httpSrv := server.NewHTTPServer(cfg, statusService, descriptorSource, logger, obs.TracingEnabled)
	grpcSrv := server.NewGRPCServer(cfg, descriptorService, workspaceService, viewService, transferService, execService, resourceContentService, logger, obs.TracingEnabled)

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
