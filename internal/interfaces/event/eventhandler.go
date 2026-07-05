package event

import (
	"context"

	chateventhandle "github.com/gonotelm-lab/gonotelm/internal/application/chat/eventhandle"
	sourceeventhandle "github.com/gonotelm-lab/gonotelm/internal/application/source/eventhandle"
	studioeventhandle "github.com/gonotelm-lab/gonotelm/internal/application/studio/eventhandle"
	"github.com/gonotelm-lab/gonotelm/internal/wire"
)

func Init(ctx context.Context, w *wire.Wire) {
	if err := registerSourceConsumers(ctx, w); err != nil {
		panic(err)
	}
	if err := registerSourceInnerConsumers(ctx, w); err != nil {
		panic(err)
	}
	if err := registerNotebookDeletedConsumers(ctx, w); err != nil {
		panic(err)
	}
}

func registerSourceConsumers(ctx context.Context, w *wire.Wire) error {
	return sourceeventhandle.RegisterPreparationConsumer(ctx,
		w.EventBus,
		sourceeventhandle.NewPrepareSourceHandler(
			w.SourceRepo,
			w.SourceStorageRepo,
			w.SourceDocRepo,
			w.Summarizer,
			w.EventBus,
		),
	)
}

func registerSourceInnerConsumers(ctx context.Context, w *wire.Wire) error {
	if err := sourceeventhandle.RegisterSourceDeletedConsumer(ctx,
		w.EventBus,
		sourceeventhandle.NewCleanupDeletedSourceHandler(
			w.SourceDocRepo,
			w.SourceStorageRepo,
		),
	); err != nil {
		return err
	}

	return nil
}

func registerNotebookDeletedConsumers(ctx context.Context, w *wire.Wire) error {
	if err := chateventhandle.RegisterNotebookDeletedConsumer(ctx,
		w.EventBus,
		chateventhandle.NewDeleteNotebookChatsHandler(
			w.ChatRepo,
			w.MessageRepo,
			w.ContextMessageRepo,
		),
	); err != nil {
		return err
	}

	if err := sourceeventhandle.RegisterNotebookDeletedConsumer(ctx,
		w.EventBus,
		sourceeventhandle.NewDeleteNotebookSourcesHandler(
			w.SourceRepo,
			w.SourceDocRepo,
			w.SourceStorageRepo,
		),
	); err != nil {
		return err
	}

	return studioeventhandle.RegisterNotebookDeletedConsumer(ctx,
		w.EventBus,
		studioeventhandle.NewDeleteNotebookArtifactTasksHandler(w.ArtifactTaskRepo),
	)
}
