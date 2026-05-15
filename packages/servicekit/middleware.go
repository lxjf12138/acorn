package servicekit

import (
	klog "github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
)

// DefaultServerMiddleware returns the baseline middleware stack for Acorn
// Kratos services.
func DefaultServerMiddleware(logger klog.Logger) []middleware.Middleware {
	return []middleware.Middleware{
		recovery.Recovery(),
		logging.Server(logger),
	}
}
