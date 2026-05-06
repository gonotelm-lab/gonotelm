package chat

import (
	"context"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	dalschema "github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
)

type Id = uuid.UUID

var ErrChatMessageNotFound = errors.New("chat message not found")

type Biz struct {
	messageStore dal.ChatMessageStore // chat message store
}

func New(messageStore dal.ChatMessageStore) *Biz {
	return &Biz{
		messageStore: messageStore,
	}
}

func (b *Biz) createMessage(
	ctx context.Context,
	chatId Id,
	userId string,
	msgType int8,
	content model.IChatMessageContent,
) (Id, error) {
	msgId := uuid.NewV7()
	createdAt := time.Now().UnixMilli()
	seqNo := time.Now().UnixNano()

	messageContent := content.Content()
	messageContent.CreatedAt = createdAt

	contentBytes, err := sonic.Marshal(messageContent)
	if err != nil {
		return msgId, errors.Wrap(errors.ErrSerde, err.Error())
	}

	err = b.messageStore.Create(ctx, &dalschema.ChatMessage{
		Id:      msgId,
		ChatId:  chatId,
		UserId:  userId,
		MsgType: msgType,
		Content: contentBytes,
		SeqNo:   seqNo,
	})
	if err != nil {
		return msgId, errors.WithMessage(err, "create chat message failed")
	}

	return msgId, nil
}

type CreateMessageCommand struct {
	NotebookId Id
	UserId     string
	Content    model.IChatMessageContent
}

func (b *Biz) createUserMessage(
	ctx context.Context,
	chatId Id,
	userId string,
	content model.IChatMessageContent,
) (Id, error) {
	content.WithRole(model.ChatMessageRoleUser)
	msgId, err := b.createMessage(
		ctx,
		chatId,
		userId,
		int8(model.ChatMessageTypeUser),
		content,
	)
	if err != nil {
		return msgId, errors.WithMessage(err, "create message failed")
	}

	return msgId, nil
}

func (b *Biz) createAssistantMessage(
	ctx context.Context,
	chatId Id,
	userId string,
	content model.IChatMessageContent,
) (Id, error) {
	content.WithRole(model.ChatMessageRoleAssistant)
	msgId, err := b.createMessage(
		ctx,
		chatId,
		userId,
		int8(model.ChatMessageTypeAssistant),
		content,
	)
	if err != nil {
		return msgId, errors.WithMessage(err, "create message failed")
	}

	return msgId, nil
}

type AddUserMessageCommand struct {
	ChatId  Id
	UserId  string
	Content string // user query text
}

// 用户发消息
func (b *Biz) AddUserMessage(
	ctx context.Context,
	cmd *AddUserMessageCommand,
) (Id, error) {
	content := model.NewChatMessageContentText(cmd.Content)
	content.WithRole(model.ChatMessageRoleUser)
	msgId, err := b.createUserMessage(
		ctx,
		cmd.ChatId,
		cmd.UserId,
		content,
	)
	if err != nil {
		return msgId, errors.WithMessage(err, "create user message failed")
	}

	return msgId, nil
}

type AddAssistantMessageCommand struct {
	ChatId           Id
	UserId           string
	Content          string // assistant response text
	ReasoningContent string // assistant reasoning content
}

func (b *Biz) AddAssistantMessage(
	ctx context.Context,
	cmd *AddAssistantMessageCommand,
) (Id, error) {
	content := model.NewChatMessageContentText(cmd.Content)
	content.WithRole(model.ChatMessageRoleAssistant)
	if cmd.ReasoningContent != "" {
		content.WithReasoningContent(&model.ChatMessageReasoningContent{Text: cmd.ReasoningContent})
	}

	msgId, err := b.createAssistantMessage(
		ctx,
		cmd.ChatId,
		cmd.UserId,
		content,
	)
	if err != nil {
		return msgId, errors.WithMessage(err, "create assistant message failed")
	}

	return msgId, nil
}

func (b *Biz) GetMessage(
	ctx context.Context,
	msgId Id,
	chatId Id,
) (*model.ChatMessage, error) {
	msg, err := b.messageStore.GetByIdAndChatId(ctx, msgId, chatId)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, ErrChatMessageNotFound
		}
		return nil, errors.WithMessage(err, "store get chat message by chat id failed")
	}

	chatMsg, err := model.NewChatMessage(msg)
	if err != nil {
		return nil, errors.WithMessage(err, "new chat message failed")
	}

	return chatMsg, nil
}

type ListMessagesQuery struct {
	ChatId Id
	UserId string
	Offset int
	Limit  int
}

func (b *Biz) ListMessages(
	ctx context.Context,
	query *ListMessagesQuery,
) ([]*model.ChatMessage, error) {
	msgs, err := b.messageStore.ListByChatId(
		ctx, query.ChatId, query.Limit, query.Offset,
	)
	if err != nil {
		return nil, errors.WithMessage(err, "store list chat messages failed")
	}

	var chatMsgs []*model.ChatMessage = make([]*model.ChatMessage, 0, len(msgs))
	for _, msg := range msgs {
		chatMsg, err := model.NewChatMessage(msg)
		if err != nil {
			slog.ErrorContext(ctx,
				"new chat message failed",
				slog.Any("err", err),
				slog.String("msg_id", msg.Id.String()),
			)
			continue
		}

		chatMsgs = append(chatMsgs, chatMsg)
	}

	return chatMsgs, nil
}
