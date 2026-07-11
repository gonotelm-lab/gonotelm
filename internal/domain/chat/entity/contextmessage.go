package entity

import (
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

type ContextMessage struct {
	Id         string
	CreateTime int64
	Message    *einoschema.Message
}

func NewUserContextMessage(chatId valobj.Id, content string) *ContextMessage {
	id := valobj.NewId()
	return &ContextMessage{
		Id:         id.String(),
		CreateTime: id.UnixMilli(),
		Message: &einoschema.Message{
			Role:    einoschema.User,
			Content: content,
		},
	}
}
