package servicekit

import (
	"os"

	klog "github.com/go-kratos/kratos/v2/log"
)

// NewLogger returns the standard service logger used by Acorn Kratos services.
func NewLogger(info BuildInfo, level string) klog.Logger {
	base := klog.NewStdLogger(os.Stdout)
	filtered := klog.NewFilter(base, klog.FilterLevel(klog.ParseLevel(level)))
	return klog.With(
		filtered,
		"ts", klog.DefaultTimestamp,
		"caller", klog.DefaultCaller,
		"service.name", info.Name,
		"service.version", info.Version,
	)
}
