package model

import (
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/bytedance/sonic"
	einoschema "github.com/cloudwego/eino/schema"
)

// 上下文消息 包含工具调用等信息
type ChatContextMessage struct {
	Id        string              `json:"id,omitempty"`
	CreatedAt int64               `json:"created_at,omitempty"`
	Message   *einoschema.Message `json:"message,omitempty"` // eino schema.Message
	Extra     []byte              `json:"extra,omitempty"`
}

func NewChatContextMessage(smsg *schema.ChatContextMessage) (*ChatContextMessage, error) {
	var message einoschema.Message
	err := sonic.Unmarshal(smsg.Message, &message)
	if err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}

	return &ChatContextMessage{
		Id:        smsg.Id,
		CreatedAt: smsg.CreatedAt,
		Message:   &message,
		Extra:     smsg.Extra,
	}, nil
}
