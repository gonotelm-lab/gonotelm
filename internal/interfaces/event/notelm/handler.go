package notelm

import (
	"context"

	artifact "github.com/gonotelm-lab/gonotelm/internal/application/notelm/artifact/eventhandle"
	chat "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/eventhandle"
	"github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/suggestion"
	pin "github.com/gonotelm-lab/gonotelm/internal/application/notelm/pin/eventhandle"
	source "github.com/gonotelm-lab/gonotelm/internal/application/notelm/source/eventhandle"
	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	adapterdefine "github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
)

type Deps struct {
	RootCtx context.Context

	NotebookRepo notebookrepo.Repository
	PinRepo      pinrepo.Repository

	SourceRepo    sourcerepo.Repository
	ObjectStore   adapterdefine.ObjectStore
	SourceDocRepo sourcerepo.SourceDocRepository

	ChatRepo               chatrepo.ChatRepository
	ChatMessageRepo        chatrepo.MessageRepository
	ChatContextMessageRepo chatrepo.ContextMessageRepository
	ChatSuggestionRepo     chatrepo.SuggestionRepository
	ChatSuggestService     *suggestion.Service

	ArtifactTaskRepo artifactrepo.Repository

	EventBus    *eventbus.CompositeEventBus
	ChatGateway *llmchat.Gateway
	FlowClient  flow.TaskClient

	Cleaner *cleanup.Cleaner
}

func Init(ctx context.Context, deps *Deps) {
	if err := initSourceEventConsumers(deps); err != nil {
		panic(err)
	}

	if err := initNotebookEventConsumers(deps); err != nil {
		panic(err)
	}

	if err := initStreamTaskEventConsumers(deps); err != nil {
		panic(err)
	}
}

func initSourceEventConsumers(deps *Deps) error {
	if err := source.RegisterSourceDeletedConsumer(
		deps.EventBus.InProcess,
		source.NewOnSourceDeletedEventHandler(deps.Cleaner),
	); err != nil {
		return err
	}

	return nil
}

func initNotebookEventConsumers(deps *Deps) error {
	if err := chat.RegisterNotebookEventConsumer(
		deps.EventBus.InProcess,
		chat.NewOnNotebookEventHandler(
			deps.ChatRepo,
			deps.ChatMessageRepo,
			deps.ChatContextMessageRepo,
		),
	); err != nil {
		return err
	}

	if err := source.RegisterNotebookEventConsumer(
		deps.EventBus.InProcess,
		source.NewOnNotebookEventHandler(deps.SourceRepo, deps.Cleaner),
	); err != nil {
		return err
	}

	if err := artifact.RegisterNotebookEventConsumer(
		deps.EventBus.InProcess,
		artifact.NewOnNotebookEventHandler(deps.ArtifactTaskRepo, deps.FlowClient, deps.Cleaner),
	); err != nil {
		return err
	}

	if err := pin.RegisterNotebookEventConsumer(
		deps.EventBus.InProcess,
		pin.NewOnNotebookEventHandler(deps.PinRepo),
	); err != nil {
		return err
	}

	return pin.RegisterNotebookMemberEventConsumer(
		deps.EventBus.InProcess,
		pin.NewOnNotebookMemberEventHandler(deps.PinRepo),
	)
}

func initStreamTaskEventConsumers(deps *Deps) error {
	if err := chat.RegisterStreamTaskEventConsumer(
		deps.EventBus.InProcess,
		chat.NewOnStreamTaskEventHandler(
			deps.RootCtx,
			deps.ChatRepo,
			deps.ChatSuggestService,
		),
	); err != nil {
		return err
	}

	return nil
}
