package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
)

type Poller interface {
	PollOne(ctx context.Context, artifactId valobj.Id)
}

type StorageGateway interface {
	DeleteObject(ctx context.Context, key string) error
	PresignGet(ctx context.Context, key string) (string, error)
}

type Deps struct {
	ArtifactRepo artifactrepo.Repository
	FlowClient   flow.TaskClient
	NotebookRepo notebookrepo.Repository
	Poller       Poller
}
