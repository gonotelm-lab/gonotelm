package chat

import (
	"context"
	"math"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatentity "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ListMessagesHandler struct {
	chatRepo        chatrepo.Repository
	chatMessageRepo chatrepo.MessageRepository
}

func NewListMessagesHandler(
	chatRepo chatrepo.Repository,
	chatMessageRepo chatrepo.MessageRepository,
) *ListMessagesHandler {
	return &ListMessagesHandler{
		chatRepo:        chatRepo,
		chatMessageRepo: chatMessageRepo,
	}
}

type ListMessagesQuery struct {
	ChatId valobj.Id
	Cursor int64
	Limit  int
}

type ListMessagesResult struct {
	Messages   []*chatentity.Message
	HasMore    bool
	NextCursor int64
}

func (h *ListMessagesHandler) Handle(
	ctx context.Context,
	query *ListMessagesQuery,
) (*ListMessagesResult, error) {
	targetChat, err := h.chatRepo.FindById(ctx, query.ChatId)
	if err != nil {
		return nil, errors.WithMessage(err, "find chat failed")
	}

	userId := pkgcontext.GetUserId(ctx)
	if targetChat.OwnerId != userId {
		return nil, errors.ErrParams.Msgf("chat not belong to user, chat_id=%s", query.ChatId)
	}

	cursor := query.Cursor
	if cursor == 0 {
		cursor = math.MaxInt64
	}
	if cursor < 0 {
		return nil, errors.ErrParams.Msgf("invalid cursor, cursor=%d", cursor)
	}

	fetchLimit := query.Limit + 1
	messages, err := h.chatMessageRepo.ListByChatIdBeforeSeqNo(ctx, query.ChatId,
		chatrepo.ListByCursorSpec{
			BeforeSeqNo: cursor,
			Limit:       fetchLimit,
		})
	if err != nil {
		return nil, errors.WithMessage(err, "list chat messages failed")
	}

	hasMore := len(messages) > query.Limit
	if hasMore {
		messages = messages[:query.Limit]
	}

	nextCursor := int64(0)
	if hasMore && len(messages) > 0 {
		nextCursor = messages[len(messages)-1].SeqNo
	}

	return &ListMessagesResult{
		Messages:   messages,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}
