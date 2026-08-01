package chat

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/suggestion"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// 获取对话聊天中的会话建议
type ChatSuggestHandler struct {
	*baseHandler
	rootCtx context.Context

	service        *suggestion.Service
	suggestionRepo chatrepo.SuggestionRepository
}

func NewChatSuggestHandler(
	rootCtx context.Context,
	chatRepo chatrepo.ChatRepository,
	suggestionRepo chatrepo.SuggestionRepository,
	messageRepo chatrepo.MessageRepository,
	messageContextRepo chatrepo.ContextMessageRepository,
	notebookRepo notebookrepo.Repository,
	sourceRepo sourcerepo.Repository,
	chatGateway *llmchat.Gateway,
) *ChatSuggestHandler {
	return &ChatSuggestHandler{
		baseHandler:    newBaseHandler(chatRepo),
		rootCtx:        rootCtx,
		suggestionRepo: suggestionRepo,
		service: suggestion.NewService(
			rootCtx,
			suggestionRepo,
			messageRepo,
			messageContextRepo,
			notebookRepo,
			sourceRepo,
			chatGateway,
		),
	}
}

type ChatSuggestCommand struct {
	ChatId    valobj.Id
	SourceIds []valobj.Id
}

type ChatSuggestResult struct {
	Questions []string
	Type      entity.SuggestionType
}

func (h *ChatSuggestHandler) Handle(ctx context.Context, cmd *ChatSuggestCommand) (*ChatSuggestResult, error) {
	targetChat, err := h.commonHandle(ctx, cmd.ChatId)
	if err != nil {
		return nil, err
	}

	// if we already have suggestions for this chat just return
	suggestions, err := h.suggestionRepo.Get(ctx, cmd.ChatId)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get suggestions",
			slog.Any("err", err), slog.String("chat_id", cmd.ChatId.String()),
		)
	}

	if suggestions != nil && len(suggestions.Questions) > 0 {
		return &ChatSuggestResult{
			Questions: suggestions.Questions,
			Type:      suggestions.Type,
		}, nil
	}

	userId := pkgcontext.GetUserId(ctx)

	result, err := h.service.Generate(ctx,
		&suggestion.GenerateSuggestionsCommand{
			Chat:      targetChat,
			SourceIds: cmd.SourceIds,
			UserId:    userId,
		})
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to generate suggestions, chat_id=%s", cmd.ChatId.String())
	}

	return &ChatSuggestResult{
		Questions: result.Questions,
		Type:      result.SuggestionType,
	}, nil
}

func (h *ChatSuggestHandler) Delete(ctx context.Context, chatId valobj.Id) {
	if _, err := h.baseHandler.commonHandle(ctx, chatId); err != nil {
		slog.WarnContext(ctx, "delete suggestions due to chat common handle failure",
			slog.Any("err", err), slog.String("chat_id", chatId.String()),
		)
		return
	}

	if err := h.suggestionRepo.Delete(ctx, chatId); err != nil {
		slog.ErrorContext(ctx, "failed to delete suggestions",
			slog.Any("err", err), slog.String("chat_id", chatId.String()),
		)
	}
}
