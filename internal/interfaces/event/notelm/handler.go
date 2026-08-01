package notelm

import (
	"context"

	artifacteventhandle "github.com/gonotelm-lab/gonotelm/internal/application/notelm/artifact/eventhandle"
	chateventhandle "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/eventhandle"
	chatsuggest "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/suggestion"
	sourceeventhandle "github.com/gonotelm-lab/gonotelm/internal/application/notelm/source/eventhandle"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
)

type EventDeps struct {
	RootCtx context.Context

	NotebookRepo notebookrepo.Repository

	SourceRepo        sourcerepo.Repository
	SourceStorageRepo sourcerepo.StorageRepository
	SourceDocRepo     sourcerepo.SourceDocRepository

	ChatRepo               chatrepo.ChatRepository
	ChatMessageRepo        chatrepo.MessageRepository
	ChatContextMessageRepo chatrepo.ContextMessageRepository
	ChatSuggestionRepo     chatrepo.SuggestionRepository
	ChatSuggestService     *chatsuggest.Service

	ArtifactTaskRepo artifactrepo.Repository

	EventBus    eventbus.EventBus
	ChatGateway *llmchat.Gateway
}

func Init(ctx context.Context, deps *EventDeps) {
	if err := initSourceEventConsumers(ctx, deps); err != nil {
		panic(err)
	}

	if err := initNotebookEventConsumers(ctx, deps); err != nil {
		panic(err)
	}

	if err := initStreamTaskEventConsumers(ctx, deps); err != nil {
		panic(err)
	}
}

func initSourceEventConsumers(ctx context.Context, deps *EventDeps) error {
	if err := sourceeventhandle.RegisterSourceDeletedConsumer(ctx,
		deps.EventBus,
		sourceeventhandle.NewOnSourceDeletedEventHandler(
			deps.SourceDocRepo,
			deps.SourceStorageRepo,
		),
	); err != nil {
		return err
	}

	return nil
}

func initNotebookEventConsumers(ctx context.Context, deps *EventDeps) error {
	if err := chateventhandle.RegisterNotebookEventConsumer(ctx,
		deps.EventBus,
		chateventhandle.NewOnNotebookEventHandler(
			deps.ChatRepo,
			deps.ChatMessageRepo,
			deps.ChatContextMessageRepo,
		),
	); err != nil {
		return err
	}

	if err := sourceeventhandle.RegisterNotebookEventConsumer(ctx,
		deps.EventBus,
		sourceeventhandle.NewOnNotebookEventHandler(
			deps.SourceRepo,
			deps.SourceDocRepo,
			deps.SourceStorageRepo,
		),
	); err != nil {
		return err
	}

	return artifacteventhandle.RegisterNotebookEventConsumer(ctx,
		deps.EventBus,
		artifacteventhandle.NewOnNotebookEventHandler(deps.ArtifactTaskRepo),
	)
}

func initStreamTaskEventConsumers(ctx context.Context, deps *EventDeps) error {
	if err := chateventhandle.RegisterStreamTaskEventConsumer(ctx,
		deps.EventBus,
		chateventhandle.NewOnStreamTaskEventHandler(
			deps.RootCtx,
			deps.ChatRepo,
			deps.ChatSuggestService,
		),
	); err != nil {
		return err
	}

	return nil
}
