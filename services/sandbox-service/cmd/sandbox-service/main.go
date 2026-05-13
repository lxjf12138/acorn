package main

import (
	"flag"
	"os"

	kratos "github.com/go-kratos/kratos/v2"
	klog "github.com/go-kratos/kratos/v2/log"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/conf"
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

	logger := newLogger(cfg)
	helper := klog.NewHelper(logger)
	statusService := service.NewStatusService()

	httpSrv := server.NewHTTPServer(cfg, statusService, logger)
	grpcSrv := server.NewGRPCServer(cfg, logger)

	app := kratos.New(
		kratos.Name(cfg.Service.Name),
		kratos.Version(version.Version),
		kratos.Logger(logger),
		kratos.Server(httpSrv, grpcSrv),
	)

	helper.Infow(
		"msg", "starting service",
		"service", cfg.Service.Name,
		"http.addr", cfg.Server.HTTP.Addr,
		"grpc.addr", cfg.Server.GRPC.Addr,
	)

	if err := app.Run(); err != nil {
		helper.Errorw("msg", "service terminated with error", "err", err)
		panic(err)
	}
}

func newLogger(cfg *conf.Config) klog.Logger {
	base := klog.NewStdLogger(os.Stdout)
	filtered := klog.NewFilter(base, klog.FilterLevel(klog.ParseLevel(cfg.Log.Level)))
	return klog.With(
		filtered,
		"ts", klog.DefaultTimestamp,
		"caller", klog.DefaultCaller,
		"service.name", cfg.Service.Name,
		"service.version", version.Version,
	)
}
