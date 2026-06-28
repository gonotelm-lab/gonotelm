package event

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/source/eventhandle"
	"github.com/gonotelm-lab/gonotelm/internal/wire"
)

func Init(ctx context.Context, wire *wire.Wire) {
	err := eventhandle.RegisterPreparationConsumer(ctx,
		wire.EventBus,
		eventhandle.NewPrepareSourceHandler(wire.SourceRepo),
	)
	if err != nil {
		panic(err)
	}
}
