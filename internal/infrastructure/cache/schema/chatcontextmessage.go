package schema

import (
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
)

// 上下文消息 包含工具调用等信息
type ChatContextMessage struct {
	Id        string `json:"a,omitempty" msgpack:"a,omitempty"`
	CreatedAt int64  `json:"b,omitempty" msgpack:"b,omitempty"`
	Message   []byte `json:"c,omitempty" msgpack:"c,omitempty"` // eino schema.Message
	Extra     []byte `json:"d,omitempty" msgpack:"d,omitempty"`
}

func (c *ChatContextMessage) ToEino() (*schema.Message, error) {
	var einoMsg schema.Message
	err := sonic.Unmarshal(c.Message, &einoMsg)
	if err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}

	return &einoMsg, nil
}
