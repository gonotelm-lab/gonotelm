package schema

import (
	"encoding/json"

	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

// 持久化对话消息
//
// 只包含用户信息和大模型回复的最终结果
type ChatMessage struct {
	Id      uuid.UUID       `gorm:"column:id"`
	ChatId  uuid.UUID       `gorm:"column:chat_id"`
	UserId  string          `gorm:"column:user_id"`
	MsgRole int8            `gorm:"column:msg_role"`
	MsgType int8            `gorm:"column:msg_type"`
	Content json.RawMessage `gorm:"column:content"` // see [model.ChatMessageContent]
	SeqNo   int64           `gorm:"column:seq_no"`
	Extra   json.RawMessage `gorm:"column:extra,omitempty"`
}
