package sourcejob

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/sourcejob/source"
	adapterdefine "github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
)

type EventDeps struct {
	SourceRepo        sourcerepo.Repository
	SourceStorageRepo sourcerepo.StorageRepository
	SourceDocRepo     sourcerepo.SourceDocRepository
	EventBus          *eventbus.CompositeEventBus
	Summarizer        adapterdefine.Summarizer
	ImageInterpreter  adapterdefine.ImageInterpreter
}

func Init(ctx context.Context, deps *EventDeps) {
	if err := source.RegisterPreparationConsumer(ctx,
		deps.EventBus.InterProcess,
		source.NewPrepareSourceHandler(
			deps.SourceRepo,
			deps.SourceStorageRepo,
			deps.SourceDocRepo,
			deps.Summarizer,
			deps.EventBus,
			deps.ImageInterpreter,
		),
	); err != nil {
		panic(err)
	}
}
