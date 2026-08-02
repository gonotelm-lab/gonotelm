package chat

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/suggestion"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// 获取对话聊天中的会话建议
type ChatSuggestHandler struct {
	*baseHandler

	service *suggestion.Service
}

func NewChatSuggestHandler(
	chatRepo chatrepo.ChatRepository,
	service *suggestion.Service,
) *ChatSuggestHandler {
	return &ChatSuggestHandler{
		baseHandler: newBaseHandler(chatRepo),
		service:     service,
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
	if len(cmd.SourceIds) == 0 {
		return &ChatSuggestResult{
			Type:      entity.SuggestionTypeNone,
			Questions: []string{},
		}, nil
	}

	targetChat, err := h.commonHandle(ctx, cmd.ChatId)
	if err != nil {
		return nil, err
	}

	// if we already have suggestions for this chat just return
	cached, err := h.service.Get(ctx, cmd.ChatId)
	if err != nil {
		return nil, err
	}

	if len(cached.Questions) > 0 {
		return &ChatSuggestResult{
			Questions: cached.Questions,
			Type:      cached.SuggestionType,
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
	if _, err := h.commonHandle(ctx, chatId); err != nil {
		slog.WarnContext(ctx, "delete suggestions due to chat common handle failure",
			slog.Any("err", err), slog.String("chat_id", chatId.String()),
		)
		return
	}

	if err := h.service.Delete(ctx, chatId); err != nil {
		slog.ErrorContext(ctx, "failed to delete suggestions",
			slog.Any("err", err), slog.String("chat_id", chatId.String()),
		)
	}
}
