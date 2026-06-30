package event

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/source/eventhandle"
	"github.com/gonotelm-lab/gonotelm/internal/wire"
)

func Init(ctx context.Context, wire *wire.Wire) {
	err := eventhandle.RegisterPreparationConsumer(ctx,
		wire.EventBus,
		eventhandle.NewPrepareSourceHandler(
			wire.SourceRepo,
			wire.SourceStorageRepo,
			wire.SourceDocRepo,
			wire.Summarizer,
		),
	)
	if err != nil {
		panic(err)
	}
}
