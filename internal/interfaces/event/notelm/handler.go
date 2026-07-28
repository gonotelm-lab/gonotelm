package notelm

import (
	"context"

	artifacteventhandle "github.com/gonotelm-lab/gonotelm/internal/application/notelm/artifact/eventhandle"
	chateventhandle "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/eventhandle"
	sourceeventhandle "github.com/gonotelm-lab/gonotelm/internal/application/notelm/source/eventhandle"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
)

type EventDeps struct {
	NotebookRepo      notebookrepo.Repository
	SourceRepo        sourcerepo.Repository
	SourceStorageRepo sourcerepo.StorageRepository
	SourceDocRepo     sourcerepo.SourceDocRepository

	ChatRepo           chatrepo.Repository
	MessageRepo        chatrepo.MessageRepository
	ContextMessageRepo chatrepo.ContextMessageRepository
	ArtifactTaskRepo   artifactrepo.Repository

	EventBus eventbus.EventBus
}

func Init(ctx context.Context, deps *EventDeps) {
	if err := registerSourceInnerConsumers(ctx, deps); err != nil {
		panic(err)
	}
	if err := registerNotebookDeletedConsumers(ctx, deps); err != nil {
		panic(err)
	}
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

	return artifacteventhandle.RegisterNotebookDeletedConsumer(ctx,
		deps.EventBus,
		artifacteventhandle.NewDeleteNotebookArtifactTasksHandler(deps.ArtifactTaskRepo),
	)
}
