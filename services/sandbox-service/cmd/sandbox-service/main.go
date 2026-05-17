package main

import (
	"flag"
	"time"

	klog "github.com/go-kratos/kratos/v2/log"
	"github.com/lxjf12138/acorn/packages/servicekit"

	"github.com/lxjf12138/acorn/packages/servicekit/localblob"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/app"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/conf"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/descriptor"
	exporteddomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/exportedresource"
	workspacedomain "github.com/lxjf12138/acorn/services/sandbox-service/internal/domain/workspace"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/infra/localfs"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/infra/localprocess"
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
		ID:      cfg.Service.ID,
		Name:    cfg.Service.Name,
		Version: version.Version,
	}, cfg.Log.Level)
	helper := klog.NewHelper(logger)
	statusService := service.NewStatusService()
	descriptorSource := descriptor.NewSourceFromConfig(cfg, version.Version)
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
	workspaceService := service.NewWorkspaceService(cfg.Service.ID, descriptorSource, workspaceStore, backingStore)
	viewService := service.NewWorkspaceViewService(cfg.Service.ID, workspaceStore, backingStore)
	transferService := service.NewWorkspaceTransferService(cfg.Service.ID, workspaceStore, backingStore, blobStore, exportStore)
	attachmentService := service.NewWorkspaceAttachmentService(workspaceStore, backingStore)
	localProcessBackend := localprocess.NewBackend(localprocess.Config{
		ID:                    "local-process-dev",
		DefaultTimeout:        time.Duration(cfg.Sandbox.LocalProcess.DefaultTimeoutSeconds) * time.Second,
		MaxTimeout:            time.Duration(cfg.Sandbox.LocalProcess.MaxTimeoutSeconds) * time.Second,
		DefaultMaxStdoutBytes: cfg.Sandbox.LocalProcess.MaxStdoutBytes,
		DefaultMaxStderrBytes: cfg.Sandbox.LocalProcess.MaxStderrBytes,
	})
	execService := service.NewWorkspaceExecService(cfg.Service.ID, workspaceStore, attachmentService, localProcessBackend)
	resourceContentService := service.NewResourceContentService(cfg.Service.ID, exportStore, blobStore)

	httpSrv := server.NewHTTPServer(cfg, statusService, descriptorSource, logger)
	grpcSrv := server.NewGRPCServer(cfg, descriptorService, workspaceService, viewService, transferService, execService, resourceContentService, logger)

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
