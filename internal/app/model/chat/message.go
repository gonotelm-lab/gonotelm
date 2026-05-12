package chat

import (
	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Id = uuid.UUID

type MessageRole int8

const (
	MessageRoleUser      MessageRole = 0
	MessageRoleAssistant MessageRole = 1
)

func (r MessageRole) String() string {
	switch r {
	case MessageRoleUser:
		return "user"
	case MessageRoleAssistant:
		return "assistant"
	default:
		return "unknown"
	}
}

type MessageType int8

const (
	MessageTypeNormal MessageType = 0
	MessageTypeSystem MessageType = 1
)

type MessageContentKind string

func (k MessageContentKind) String() string {
	return string(k)
}

const (
	MessageContentKindText MessageContentKind = "text"
)

type Message struct {
	Id      Id
	ChatId  Id
	UserId  string
	MsgRole MessageRole
	MsgType MessageType
	Content *MessageContent
	SeqNo   int64
	Extra   *MessageExtra
}

type MessageExtra struct {
	Citation []*Citation `json:"citation,omitempty"`
}

type Citation struct {
	// 按照sourceId分组
	SourceId string   `json:"source_id"`
	DocIds   []string `json:"doc_ids,omitempty"`
}

func NewMessage(smsg *schema.ChatMessage) (*Message, error) {
	var content MessageContent
	err := sonic.Unmarshal(smsg.Content, &content)
	if err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}

	var extra *MessageExtra
	if len(smsg.Extra) > 0 {
		extra = &MessageExtra{}
		err = sonic.Unmarshal(smsg.Extra, extra)
		if err != nil {
			return nil, errors.Wrap(errors.ErrSerde, err.Error())
		}
	}

	return &Message{
		Id:      smsg.Id,
		ChatId:  smsg.ChatId,
		UserId:  smsg.UserId,
		MsgRole: MessageRole(smsg.MsgRole),
		MsgType: MessageType(smsg.MsgType),
		Content: &content,
		SeqNo:   smsg.SeqNo,
		Extra:   extra,
	}, nil
}

type MessageContent struct {
	Role      string              `json:"role"` // 消息角色
	CreatedAt int64               `json:"created_at"`
	Kind      string              `json:"kind"`           // 消息类型
	Text      *MessageContentText `json:"text,omitempty"` // 文本消息

	ReasoningContent *MessageReasoningContent `json:"reasoning_content,omitempty"` // 模型推理内容
}

type MessageReasoningContent struct {
	Content string `json:"content,omitempty"`
}

type IBaseMessageContent interface {
	WithRole(role MessageRole)
	WithReasoningContent(reasoningContent *MessageReasoningContent)
}

type IMessageContent interface {
	IBaseMessageContent

	Kind() MessageContentKind
	GetMessageContent() *MessageContent
}

type BaseMessageContent struct {
	role             MessageRole
	reasoningContent *MessageReasoningContent
}

func (c *BaseMessageContent) WithRole(role MessageRole) {
	c.role = role
}

func (c *BaseMessageContent) WithReasoningContent(reasoningContent *MessageReasoningContent) {
	c.reasoningContent = reasoningContent
}

var _ IMessageContent = &MessageContentText{}

type MessageContentText struct {
	*BaseMessageContent `json:"-"`

	Content string `json:"content"`
}

func NewMessageContentText(text string) *MessageContentText {
	return &MessageContentText{
		BaseMessageContent: &BaseMessageContent{},
		Content:            text,
	}
}

func (c *MessageContentText) Kind() MessageContentKind {
	return MessageContentKindText
}

func (c *MessageContentText) GetMessageContent() *MessageContent {
	return &MessageContent{
		Role:             c.role.String(),
		ReasoningContent: c.reasoningContent,
		Kind:             c.Kind().String(),
		Text:             &MessageContentText{Content: c.Content},
	}
}
