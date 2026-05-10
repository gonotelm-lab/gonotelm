package chat

import (
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/bytedance/sonic"
	einoschema "github.com/cloudwego/eino/schema"
)

// 上下文消息 包含工具调用等信息
type ContextMessage struct {
	Id        string              `json:"id,omitempty"`
	CreatedAt int64               `json:"created_at,omitempty"`
	Message   *einoschema.Message `json:"message,omitempty"` // eino schema.Message
	Extra     []byte              `json:"extra,omitempty"`
}

func NewContextMessage(smsg *schema.ChatContextMessage) (*ContextMessage, error) {
	var message einoschema.Message
	err := sonic.Unmarshal(smsg.Message, &message)
	if err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}

	return &ContextMessage{
		Id:        smsg.Id,
		CreatedAt: smsg.CreatedAt,
		Message:   &message,
		Extra:     smsg.Extra,
	}, nil
}
