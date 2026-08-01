package eventhandle

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/suggestion"
	chatevent "github.com/gonotelm-lab/gonotelm/internal/domain/chat/event"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/safe"
)

type OnStreamTaskEventHandler struct {
	rootCtx context.Context

	chatRepo chatrepo.ChatRepository

	suggestionService *suggestion.Service
}

func NewOnStreamTaskEventHandler(
	rootCtx context.Context,
	chatRepo chatrepo.ChatRepository,
	messageRepo chatrepo.MessageRepository,
	contextMessageRepo chatrepo.ContextMessageRepository,
	suggestionRepo chatrepo.SuggestionRepository,
	notebookRepo notebookrepo.Repository,
	sourceRepo sourcerepo.Repository,
	chatGateway *llmchat.Gateway,
) *OnStreamTaskEventHandler {
	return &OnStreamTaskEventHandler{
		rootCtx:  rootCtx,
		chatRepo: chatRepo,
		suggestionService: suggestion.NewService(
			rootCtx,
			suggestionRepo,
			messageRepo,
			contextMessageRepo,
			notebookRepo,
			sourceRepo,
			chatGateway,
		),
	}
}

func (h *OnStreamTaskEventHandler) Handle(
	ctx context.Context,
	evt *chatevent.StreamTaskEvent,
) error {
	switch evt.Action() {
	case chatevent.StreamTaskEventActionFinish:
		return h.handleFinish(ctx, evt)
	case chatevent.StreamTaskEventActionAbort:
		return h.handleAbort(ctx, evt)
	default:
		return errors.New("invalid stream task event action")
	}
}

// 回答完毕后 提前触发一次下轮问题的建议给用户
func (h *OnStreamTaskEventHandler) handleFinish(ctx context.Context, evt *chatevent.StreamTaskEvent) error {
	slog.DebugContext(ctx, "chat handling stream task finished event",
		slog.String("chat_id", evt.ChatId().String()),
		slog.String("task_id", evt.TaskId().String()),
	)

	safe.DetachGo(ctx, h.rootCtx, func(ctx context.Context) {
		h.generateSuggestions(ctx, evt)
	})

	return nil
}

func (h *OnStreamTaskEventHandler) generateSuggestions(
	ctx context.Context,
	evt *chatevent.StreamTaskEvent,
) {
	// get target chat
	targetChat, err := h.chatRepo.FindById(ctx, evt.ChatId())
	if err != nil {
		slog.ErrorContext(ctx, "suggestion failed to get target chat",
			slog.Any("err", err),
			slog.String("chat_id", evt.ChatId().String()),
			slog.String("task_id", evt.TaskId().String()),
		)
		return
	}

	_, err = h.suggestionService.Generate(
		ctx,
		&suggestion.GenerateSuggestionsCommand{
			Chat:      targetChat,
			SourceIds: evt.SourceIds(),
			UserId:    targetChat.OwnerId,
		},
	)
	if err != nil {
		slog.ErrorContext(ctx, "suggestion failed to generate",
			slog.Any("err", err),
			slog.String("chat_id", evt.ChatId().String()),
			slog.String("task_id", evt.TaskId().String()),
		)
	}
}

func (h *OnStreamTaskEventHandler) handleAbort(ctx context.Context, evt *chatevent.StreamTaskEvent) error {
	slog.DebugContext(ctx, "chat handling stream task aborted event",
		slog.String("chat_id", evt.ChatId().String()),
		slog.String("task_id", evt.TaskId().String()),
	)

	return nil
}

func RegisterStreamTaskEventConsumer(
	ctx context.Context,
	bus eventbus.EventBus,
	handler *OnStreamTaskEventHandler,
) error {
	return eventbus.SubscribeStreamTaskEvent(ctx, bus, handler.Handle)
}
