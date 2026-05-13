package server

import (
	nethttp "net/http"

	klog "github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/conf"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/service"
)

func NewHTTPServer(cfg *conf.Config, statusService *service.StatusService, logger klog.Logger) *khttp.Server {
	srv := khttp.NewServer(
		khttp.Address(cfg.Server.HTTP.Addr),
		khttp.Timeout(cfg.Server.HTTP.TimeoutDuration()),
		khttp.Middleware(
			recovery.Recovery(),
			logging.Server(logger),
		),
	)

	router := srv.Route("/")
	router.GET("/healthz", func(ctx khttp.Context) error {
		return ctx.JSON(nethttp.StatusOK, statusService.Health())
	})
	router.GET("/readyz", func(ctx khttp.Context) error {
		return ctx.JSON(nethttp.StatusOK, statusService.Ready())
	})
	router.GET("/version", func(ctx khttp.Context) error {
		return ctx.JSON(nethttp.StatusOK, statusService.Version())
	})

	return srv
}
