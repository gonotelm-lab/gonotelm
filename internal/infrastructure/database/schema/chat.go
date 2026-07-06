package schema

import "github.com/gonotelm-lab/gonotelm/pkg/uuid"

type Chat struct {
	Id         uuid.UUID `gorm:"column:id"`
	NotebookId uuid.UUID `gorm:"column:notebook_id"`
	OwnerId    string    `gorm:"column:owner_id"`
	UpdatedAt  int64     `gorm:"column:updated_at"`
}

func (Chat) TableName() string {
	return "chats"
}
