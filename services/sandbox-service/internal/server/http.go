package server

import (
	nethttp "net/http"

	klog "github.com/go-kratos/kratos/v2/log"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/lxjf12138/acorn/packages/servicekit"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/lxjf12138/acorn/services/sandbox-service/internal/conf"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/descriptor"
	"github.com/lxjf12138/acorn/services/sandbox-service/internal/service"
)

func NewHTTPServer(cfg *conf.Config, statusService *service.StatusService, descriptorSource *descriptor.Source, logger klog.Logger, tracingEnabled bool) *khttp.Server {
	srv := khttp.NewServer(
		khttp.Address(cfg.Server.HTTP.Addr),
		khttp.Timeout(cfg.Server.HTTP.TimeoutDuration()),
		khttp.Middleware(servicekit.DefaultServerMiddleware(servicekit.ServerMiddlewareOptions{
			Logger:         logger,
			TracingEnabled: tracingEnabled,
		})...),
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
	router.GET("/capability/descriptor", func(ctx khttp.Context) error {
		capabilityDescriptor, err := descriptorSource.DescribeCapabilities(ctx)
		if err != nil {
			return err
		}
		body, err := protojson.MarshalOptions{
			UseProtoNames:   true,
			EmitUnpopulated: true,
		}.Marshal(capabilityDescriptor)
		if err != nil {
			return err
		}
		return ctx.Blob(nethttp.StatusOK, "application/json", body)
	})

	return srv
}
