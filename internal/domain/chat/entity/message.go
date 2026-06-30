package entity

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/entity"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

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

type MessageContentKind string

func (k MessageContentKind) String() string {
	return string(k)
}

const (
	MessageContentKindText MessageContentKind = "text"
)

type Message struct {
	entity.Base

	ChatId           valobj.Id
	UserId           string
	Role             MessageRole
	Content          MessageContent
	ReasoningContent *MessageReasoningContent
	SeqNo            int64
}

type MessageContent interface {
	Bytes() ([]byte, error)
	Kind() MessageContentKind
}

type MessageReasoningContent struct {
	Content string
}
