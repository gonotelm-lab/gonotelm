package sourcejob

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/sourcejob/prepare"
	adapterdefine "github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
)

type EventDeps struct {
	SourceRepo        sourcerepo.Repository
	SourceStorageRepo sourcerepo.StorageRepository
	SourceDocRepo     sourcerepo.SourceDocRepository
	EventBus          eventbus.EventBus
	Summarizer        adapterdefine.Summarizer
}

func Init(ctx context.Context, deps *EventDeps) {
	if err := prepare.RegisterPreparationConsumer(ctx,
		deps.EventBus,
		prepare.NewPrepareSourceHandler(
			deps.SourceRepo,
			deps.SourceStorageRepo,
			deps.SourceDocRepo,
			deps.Summarizer,
			deps.EventBus,
		),
	); err != nil {
		panic(err)
	}
}
