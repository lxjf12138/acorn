package servicekit

import (
	klog "github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
)

type ServerMiddlewareOptions struct {
	Logger klog.Logger

	TracingEnabled bool
	MetricsEnabled bool
}

type ClientMiddlewareOptions struct {
	TracingEnabled bool
	MetricsEnabled bool
}

// DefaultServerMiddleware returns the baseline middleware stack for Acorn
// Kratos services.
func DefaultServerMiddleware(opts ServerMiddlewareOptions) []middleware.Middleware {
	out := []middleware.Middleware{
		recovery.Recovery(),
	}
	if opts.TracingEnabled {
		out = append(out, tracing.Server())
	}
	out = append(out, logging.Server(opts.Logger))
	return out
}

func DefaultClientMiddleware(opts ClientMiddlewareOptions) []middleware.Middleware {
	var out []middleware.Middleware
	if opts.TracingEnabled {
		out = append(out, tracing.Client())
	}
	return out
}
