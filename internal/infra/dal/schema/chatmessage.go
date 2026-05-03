package schema

import (
	"encoding/json"

	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type ChatMessage struct {
	Id      uuid.UUID       `gorm:"column:id"`
	ChatId  uuid.UUID       `gorm:"column:chat_id"`
	UserId  string          `gorm:"column:user_id"`
	Role    string          `gorm:"column:role"`
	Content json.RawMessage `gorm:"column:content"`
	SeqNo   int64           `gorm:"column:seq_no"`
}
