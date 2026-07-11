package schema

import (
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Artifact struct {
	Id         uuid.UUID `gorm:"column:id;primaryKey"`
	NotebookId uuid.UUID `gorm:"column:notebook_id"`
	UserId     string    `gorm:"column:user_id"`
	Kind       string    `gorm:"column:kind"`
	Status     string    `gorm:"column:status"`
	FlowTaskId string    `gorm:"column:flow_task_id"`
	Title      string    `gorm:"column:title"`
	Result     []byte    `gorm:"column:result"`
	ResultKind string    `gorm:"column:result_kind"`
	Payload    []byte    `gorm:"column:payload"`
	CreatedAt  int64     `gorm:"column:created_at"`
	UpdatedAt  int64     `gorm:"column:updated_at"`
}

func (Artifact) TableName() string { return "artifacts" }
