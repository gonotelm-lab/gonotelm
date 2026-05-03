package schema

import "github.com/gonotelm-lab/gonotelm/pkg/uuid"

type Notebook struct {
	Id          uuid.UUID `gorm:"column:id"`
	Name        string    `gorm:"column:name"`
	Description string    `gorm:"column:description"`
	OwnerId     string    `gorm:"column:owner_id"`
	UpdatedAt   int64     `gorm:"column:updated_at"`
}

func (Notebook) TableName() string {
	return "notebooks"
}
