package model

import (
	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ChatMessageRole string

func (r ChatMessageRole) String() string {
	return string(r)
}

const (
	ChatMessageRoleUser      ChatMessageRole = "user"
	ChatMessageRoleAssistant ChatMessageRole = "assistant"
)

type ChatMessageType int8

const (
	ChatMessageTypeUser      ChatMessageType = 0
	ChatMessageTypeAssistant ChatMessageType = 1
)

type ChatMessageContentKind string

func (k ChatMessageContentKind) String() string {
	return string(k)
}

const (
	ChatMessageContentKindText ChatMessageContentKind = "text"
)

type ChatMessage struct {
	Id      Id
	ChatId  Id
	UserId  string
	MsgType ChatMessageType
	Content *ChatMessageContent
	SeqNo   int64
	Extra   []byte
}

func NewChatMessage(smsg *schema.ChatMessage) (*ChatMessage, error) {
	var content ChatMessageContent
	err := sonic.Unmarshal(smsg.Content, &content)
	if err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}

	return &ChatMessage{
		Id:      smsg.Id,
		ChatId:  smsg.ChatId,
		UserId:  smsg.UserId,
		MsgType: ChatMessageType(smsg.MsgType),
		Content: &content,
		SeqNo:   smsg.SeqNo,
		Extra:   smsg.Extra,
	}, nil
}

type ChatMessageContent struct {
	Role      string                  `json:"role"` // 消息角色
	CreatedAt int64                   `json:"created_at"`
	Kind      string                  `json:"kind"`           // 消息类型
	Text      *ChatMessageContentText `json:"text,omitempty"` // 文本消息

	ReasoningContent *ChatMessageReasoningContent `json:"reasoning_content,omitempty"` // 模型推理过程
}

type ChatMessageReasoningContent struct {
	Text string `json:"text"`
}

type IBaseChatMessageContent interface {
	WithRole(role ChatMessageRole)
	WithReasoningContent(reasoningContent *ChatMessageReasoningContent)
}

type IChatMessageContent interface {
	IBaseChatMessageContent

	Kind() ChatMessageContentKind
	Content() *ChatMessageContent
}

type BaseChatMessageContent struct {
	role             ChatMessageRole
	reasoningContent *ChatMessageReasoningContent
}

func (c *BaseChatMessageContent) WithRole(role ChatMessageRole) {
	c.role = role
}

func (c *BaseChatMessageContent) WithReasoningContent(reasoningContent *ChatMessageReasoningContent) {
	c.reasoningContent = reasoningContent
}

var _ IChatMessageContent = &ChatMessageContentText{}

type ChatMessageContentText struct {
	*BaseChatMessageContent
	Text string
}

func NewChatMessageContentText(text string) *ChatMessageContentText {
	return &ChatMessageContentText{Text: text}
}

func (c *ChatMessageContentText) Kind() ChatMessageContentKind {
	return ChatMessageContentKindText
}

func (c *ChatMessageContentText) Content() *ChatMessageContent {
	return &ChatMessageContent{
		Role:             c.role.String(),
		ReasoningContent: c.reasoningContent,
		Kind:             c.Kind().String(),
		Text:             &ChatMessageContentText{Text: c.Text},
	}
}
