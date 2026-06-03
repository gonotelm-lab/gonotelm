package artifact

import "github.com/gonotelm-lab/gonotelm/internal/infra/dal"

type Biz struct {
	taskStore dal.ArtifactTaskStore
}

func New(taskStore dal.ArtifactTaskStore) *Biz {
	return &Biz{taskStore: taskStore}
}
