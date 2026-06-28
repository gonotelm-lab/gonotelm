package wire

import (
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source"
	"github.com/gonotelm-lab/gonotelm/internal/infra"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository"
)

var gWire *Wire

type Wire struct {
	NotebookRepo      notebook.Repository
	SourceRepo        source.Repository
	SourceStorageRepo source.StorageRepository

	EventBus eventbus.EventBus
}

func Init(infras *infra.Instances) {
	notebookRepo := repository.NewNotebookRepository(
		infras.Dal.NotebookStore,
		infras.Dal.SourceStore,
	)
	sourceRepo := repository.NewSourceRepository(
		infras.Dal.SourceStore,
	)
	sourceStorageRepo := repository.NewSourceStorageRepository(infras.ObjectStorage)

	gWire = &Wire{
		NotebookRepo:      notebookRepo,
		SourceRepo:        sourceRepo,
		SourceStorageRepo: sourceStorageRepo,
		EventBus:          eventbus.NewOuterEventBus(infras.MQ),
	}
}

func GetWire() *Wire {
	return gWire
}
