package schema

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type NotebookMember struct {
	Id         uuid.UUID  `gorm:"column:id;primaryKey"`
	NotebookId uuid.UUID  `gorm:"column:notebook_id"`
	UserId     valobj.Uid `gorm:"column:user_id"`
	Role       string     `gorm:"column:role"`
	CreatedAt  int64      `gorm:"column:created_at"`
	UpdatedAt  int64      `gorm:"column:updated_at"`
}

func (NotebookMember) TableName() string {
	return "notebook_members"
}
