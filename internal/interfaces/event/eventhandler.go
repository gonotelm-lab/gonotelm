package event

import (
	"context"

	chateventhandle "github.com/gonotelm-lab/gonotelm/internal/application/chat/eventhandle"
	sourceeventhandle "github.com/gonotelm-lab/gonotelm/internal/application/source/eventhandle"
	studioeventhandle "github.com/gonotelm-lab/gonotelm/internal/application/studio/eventhandle"
	adapterdefine "github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository"
)

type EventDeps struct {
	NotebookRepo      notebookrepo.Repository
	SourceRepo        sourcerepo.Repository
	SourceStorageRepo sourcerepo.StorageRepository
	SourceDocRepo     sourcerepo.SourceDocRepository

	ChatRepo           chatrepo.Repository
	MessageRepo        chatrepo.MessageRepository
	ContextMessageRepo chatrepo.ContextMessageRepository
	ArtifactTaskRepo   *repository.ArtifactTaskRepository

	EventBus eventbus.EventBus

	Summarizer adapterdefine.Summarizer
}

func Init(ctx context.Context, deps *EventDeps) {
	if err := registerSourceConsumers(ctx, deps); err != nil {
		panic(err)
	}
	if err := registerSourceInnerConsumers(ctx, deps); err != nil {
		panic(err)
	}
	if err := registerNotebookDeletedConsumers(ctx, deps); err != nil {
		panic(err)
	}
}

func registerSourceConsumers(ctx context.Context, deps *EventDeps) error {
	return sourceeventhandle.RegisterPreparationConsumer(ctx,
		deps.EventBus,
		sourceeventhandle.NewPrepareSourceHandler(
			deps.SourceRepo,
			deps.SourceStorageRepo,
			deps.SourceDocRepo,
			deps.Summarizer,
			deps.EventBus,
		),
	)
}

func registerSourceInnerConsumers(ctx context.Context, deps *EventDeps) error {
	if err := sourceeventhandle.RegisterSourceDeletedConsumer(ctx,
		deps.EventBus,
		sourceeventhandle.NewCleanupDeletedSourceHandler(
			deps.SourceDocRepo,
			deps.SourceStorageRepo,
		),
	); err != nil {
		return err
	}

	return nil
}

func registerNotebookDeletedConsumers(ctx context.Context, deps *EventDeps) error {
	if err := chateventhandle.RegisterNotebookDeletedConsumer(ctx,
		deps.EventBus,
		chateventhandle.NewDeleteNotebookChatsHandler(
			deps.ChatRepo,
			deps.MessageRepo,
			deps.ContextMessageRepo,
		),
	); err != nil {
		return err
	}

	if err := sourceeventhandle.RegisterNotebookDeletedConsumer(ctx,
		deps.EventBus,
		sourceeventhandle.NewDeleteNotebookSourcesHandler(
			deps.SourceRepo,
			deps.SourceDocRepo,
			deps.SourceStorageRepo,
		),
	); err != nil {
		return err
	}

	return studioeventhandle.RegisterNotebookDeletedConsumer(ctx,
		deps.EventBus,
		studioeventhandle.NewDeleteNotebookArtifactTasksHandler(deps.ArtifactTaskRepo),
	)
}
