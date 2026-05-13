package signal

import (
	"context"

	signalv1 "github.com/lxjf12138/acorn/packages/api/gen/acorn/signal/v1"
)

type Ingest interface {
	Emit(ctx context.Context, sig *signalv1.Signal) error
}
