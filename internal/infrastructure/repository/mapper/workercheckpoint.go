package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func CheckpointToSchema(cp *entity.Checkpoint) *schema.WorkerCheckpoint {
	return &schema.WorkerCheckpoint{
		ArtifactId: uuid.UUID(cp.ArtifactId),
		Field1:     cp.Field1,
		Field2:     cp.Field2,
		Field3:     cp.Field3,
		Field4:     cp.Field4,
		Field5:     cp.Field5,
		Field6:     cp.Field6,
		Field7:     cp.Field7,
		Field8:     cp.Field8,
		CreatedAt:  cp.CreateTime.Value(),
		UpdatedAt:  cp.UpdateTime.Value(),
	}
}

func CheckpointFromSchema(sch *schema.WorkerCheckpoint) *entity.Checkpoint {
	return &entity.Checkpoint{
		ArtifactId: valobj.Id(sch.ArtifactId),
		Field1:     sch.Field1,
		Field2:     sch.Field2,
		Field3:     sch.Field3,
		Field4:     sch.Field4,
		Field5:     sch.Field5,
		Field6:     sch.Field6,
		Field7:     sch.Field7,
		Field8:     sch.Field8,
		CreateTime: valobj.NewTimeFrom(sch.CreatedAt),
		UpdateTime: valobj.NewTimeFrom(sch.UpdatedAt),
	}
}
