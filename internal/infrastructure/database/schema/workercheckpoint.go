package schema

import "github.com/gonotelm-lab/gonotelm/pkg/uuid"

type WorkerCheckpoint struct {
	ArtifactId uuid.UUID `gorm:"column:artifact_id;primaryKey"`
	Field1     []byte    `gorm:"column:field1"`
	Field2     []byte    `gorm:"column:field2"`
	Field3     []byte    `gorm:"column:field3"`
	Field4     []byte    `gorm:"column:field4"`
	Field5     []byte    `gorm:"column:field5"`
	Field6     []byte    `gorm:"column:field6"`
	Field7     []byte    `gorm:"column:field7"`
	Field8     []byte    `gorm:"column:field8"`
	CreatedAt  int64     `gorm:"column:created_at"`
	UpdatedAt  int64     `gorm:"column:updated_at"`
}

func (WorkerCheckpoint) TableName() string {
	return "worker_artifact_checkpoints"
}
