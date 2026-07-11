package schema

import (
	"time"

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
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (Artifact) TableName() string { return "artifacts" }
