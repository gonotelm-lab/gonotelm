package chat

import (
	"context"
	"log/slog"
	"time"

	chatmodel "github.com/gonotelm-lab/gonotelm/internal/app/model/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache"
	cacheschema "github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	dalschema "github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
	einoschema "github.com/cloudwego/eino/schema"
)

type Id = uuid.UUID

var ErrChatMessageNotFound = errors.New("chat message not found")

type Biz struct {
	messageStore dal.ChatMessageStore          // chat message store
	contextStore cache.ChatContextMessageCache // chat context message cache store
}

func New(
	messageStore dal.ChatMessageStore,
	contextStore cache.ChatContextMessageCache,
) *Biz {
	return &Biz{
		messageStore: messageStore,
		contextStore: contextStore,
	}
}

func (b *Biz) createMessage(
	ctx context.Context,
	chatId Id,
	userId string,
	msgRole int8,
	msgType int8,
	content chatmodel.IMessageContent,
) (Id, error) {
	msgId := uuid.NewV7()
	createdAt := time.Now().UnixMilli()
	seqNo := time.Now().UnixNano()

	messageContent := content.GetMessageContent()
	messageContent.CreatedAt = createdAt

	contentBytes, err := sonic.Marshal(messageContent)
	if err != nil {
		return msgId, errors.Wrap(errors.ErrSerde, err.Error())
	}

	err = b.messageStore.Create(ctx, &dalschema.ChatMessage{
		Id:      msgId,
		ChatId:  chatId,
		UserId:  userId,
		MsgRole: msgRole,
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
	Content    chatmodel.IMessageContent
}

func (b *Biz) createUserMessage(
	ctx context.Context,
	chatId Id,
	userId string,
	content chatmodel.IMessageContent,
) (Id, error) {
	content.WithRole(chatmodel.MessageRoleUser)
	msgId, err := b.createMessage(
		ctx,
		chatId,
		userId,
		int8(chatmodel.MessageRoleUser),
		int8(chatmodel.MessageTypeNormal),
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
	content chatmodel.IMessageContent,
) (Id, error) {
	content.WithRole(chatmodel.MessageRoleAssistant)
	msgId, err := b.createMessage(
		ctx,
		chatId,
		userId,
		int8(chatmodel.MessageRoleAssistant),
		int8(chatmodel.MessageTypeNormal),
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
	content := chatmodel.NewMessageContentText(cmd.Content)
	content.WithRole(chatmodel.MessageRoleUser)
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
	ReasoningContent *chatmodel.MessageReasoningContent // assistant reasoning content
}

func (b *Biz) AddAssistantMessage(
	ctx context.Context,
	cmd *AddAssistantMessageCommand,
) (Id, error) {
	content := chatmodel.NewMessageContentText(cmd.Content)
	content.WithRole(chatmodel.MessageRoleAssistant)
	if cmd.ReasoningContent != nil {
		content.WithReasoningContent(cmd.ReasoningContent)
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

type AddAssistantSystemMessageCommand struct {
	ChatId  Id
	UserId  string
	Content string
}

func (b *Biz) AddAssistantSystemMessage(
	ctx context.Context,
	cmd *AddAssistantSystemMessageCommand,
) (Id, error) {
	content := chatmodel.NewMessageContentText(cmd.Content)
	content.WithRole(chatmodel.MessageRoleAssistant)

	msgId, err := b.createMessage(
		ctx,
		cmd.ChatId,
		cmd.UserId,
		int8(chatmodel.MessageRoleAssistant),
		int8(chatmodel.MessageTypeSystem),
		content,
	)
	if err != nil {
		return msgId, errors.WithMessage(err, "create assistant system message failed")
	}

	return msgId, nil
}

func (b *Biz) GetMessage(
	ctx context.Context,
	msgId Id,
	chatId Id,
) (*chatmodel.Message, error) {
	msg, err := b.messageStore.GetByIdAndChatId(ctx, msgId, chatId)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, ErrChatMessageNotFound
		}
		return nil, errors.WithMessage(err, "store get chat message by chat id failed")
	}

	chatMsg, err := chatmodel.NewMessage(msg)
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
) ([]*chatmodel.Message, error) {
	msgs, err := b.messageStore.ListByChatId(
		ctx, query.ChatId, query.Limit, query.Offset,
	)
	if err != nil {
		return nil, errors.WithMessage(err, "store list chat messages failed")
	}

	var chatMsgs []*chatmodel.Message = make([]*chatmodel.Message, 0, len(msgs))
	for _, msg := range msgs {
		chatMsg, err := chatmodel.NewMessage(msg)
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

func (b *Biz) AppendContextMessage(
	ctx context.Context,
	chatId Id,
	messages []*einoschema.Message,
) error {
	if len(messages) == 0 {
		return nil
	}

	now := time.Now().UnixMilli()
	ctxMsgs := make([]*cacheschema.ChatContextMessage, 0, len(messages))
	for _, msg := range messages {
		raw, err := sonic.Marshal(msg)
		if err != nil {
			return errors.Wrap(errors.ErrSerde, err.Error())
		}

		ctxMsgs = append(ctxMsgs, &cacheschema.ChatContextMessage{
			Id:        uuid.NewV7().String(),
			CreatedAt: now,
			Message:   raw,
		})
	}

	err := b.contextStore.Append(ctx, chatId.String(), ctxMsgs)
	if err != nil {
		return errors.WithMessage(err, "append context message failed")
	}

	return nil
}

func (b *Biz) ListContextMessages(
	ctx context.Context,
	chatId Id,
) ([]*einoschema.Message, error) {
	ctxMsgs, err := b.contextStore.ListAll(ctx, chatId.String())
	if err != nil {
		return nil, errors.WithMessage(err, "list context messages failed")
	}

	einoMsgs := make([]*einoschema.Message, 0, len(ctxMsgs))
	for _, ctxMsg := range ctxMsgs {
		einoMsg, err := ctxMsg.ToEino()
		if err != nil {
			return nil, errors.Wrap(errors.ErrSerde, err.Error())
		}

		einoMsgs = append(einoMsgs, einoMsg)
	}

	return einoMsgs, nil
}
