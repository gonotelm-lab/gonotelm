package schema

import "github.com/gonotelm-lab/gonotelm/pkg/uuid"

const (
	SourceStatusInited = "inited"
)

type Source struct {
	Id          uuid.UUID `gorm:"column:id"`
	NotebookId  uuid.UUID `gorm:"column:notebook_id"`
	Kind        string    `gorm:"column:kind"`
	Status      string    `gorm:"column:status"`
	DisplayName string    `gorm:"column:display_name"`
	Content     []byte    `gorm:"column:content"`
	OwnerId     string    `gorm:"column:owner_id"`
	UpdatedAt   int64     `gorm:"column:updated_at"`
}

func (Source) TableName() string {
	return "sources"
}

type SourceUpdateParams struct {
	Id          uuid.UUID
	Status      string
	DisplayName string
	Content     []byte
	UpdatedAt   int64
}
