package sourcejob

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/sourcejob/source"
	adapterdefine "github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
)

type EventDeps struct {
	SourceRepo       sourcerepo.Repository
	NotebookRepo     notebookrepo.Repository
	ObjectStore      adapterdefine.ObjectStore
	KeyFactory       adapterdefine.StoreKeyFactory
	SourceDocRepo    sourcerepo.SourceDocRepository
	EventBus         *eventbus.CompositeEventBus
	Summarizer       adapterdefine.Summarizer
	ImageInterpreter adapterdefine.ImageInterpreter
}

func Init(ctx context.Context, deps *EventDeps) {
	if err := source.RegisterPreparationConsumer(ctx,
		deps.EventBus.InterProcess,
		source.NewPrepareSourceHandler(
			deps.SourceRepo,
			deps.NotebookRepo,
			deps.ObjectStore,
			deps.KeyFactory,
			deps.SourceDocRepo,
			deps.Summarizer,
			deps.EventBus,
			deps.ImageInterpreter,
		),
	); err != nil {
		panic(err)
	}
}
