package app

import (
	kratos "github.com/go-kratos/kratos/v2"
	klog "github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
)

func New(name string, version string, logger klog.Logger, servers ...transport.Server) *kratos.App {
	return kratos.New(
		kratos.Name(name),
		kratos.Version(version),
		kratos.Logger(logger),
		kratos.Server(servers...),
	)
}
