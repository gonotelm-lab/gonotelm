package schema

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type UserNotebookPin struct {
	UserId     valobj.Uid `gorm:"column:user_id;primaryKey"`
	NotebookId uuid.UUID  `gorm:"column:notebook_id;primaryKey"`
	PinnedAt   int64      `gorm:"column:pinned_at"`
}

func (UserNotebookPin) TableName() string {
	return "user_notebook_pins"
}
