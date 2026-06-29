package wire

import (
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infra"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/embedding"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository"
)

var gWire *Wire

type Wire struct {
	NotebookRepo      notebookrepo.Repository
	SourceRepo        sourcerepo.Repository
	SourceStorageRepo sourcerepo.StorageRepository
	SourceDocRepo     sourcerepo.SourceDocRepository

	EventBus eventbus.EventBus
}

func Init(infras *infra.Instances) {
	embedGateway, err := embedding.NewGateway(
		&conf.Global().Embedding,
		embedding.NewRedisCacher(infras.Redis),
	)
	if err != nil {
		panic(err)
	}

	embedder, err := embedGateway.GetProvider(conf.Global().Embedding.Type)
	if err != nil {
		panic(err)
	}

	notebookRepo := repository.NewNotebookRepository(
		infras.Dal.NotebookStore,
		infras.Dal.SourceStore,
	)
	sourceRepo := repository.NewSourceRepository(
		infras.Dal.SourceStore,
	)
	sourceStorageRepo := repository.NewSourceStorageRepository(infras.ObjectStorage)
	sourceDocRepo := repository.NewSourceDocRepository(
		embedder,
		infras.VectorDal.SourceDocStore,
		repository.SourceDocRepositoryConfig{
			EmbedBatchSize:      conf.Global().Embedding.BatchSize,
			EmbedMaxConcurrency: conf.Global().Embedding.MaxConcurrency,
		})

	gWire = &Wire{
		NotebookRepo:      notebookRepo,
		SourceRepo:        sourceRepo,
		SourceStorageRepo: sourceStorageRepo,
		SourceDocRepo:     sourceDocRepo,
		EventBus:          eventbus.NewOuterEventBus(infras.MQ),
	}
}

func GetWire() *Wire {
	return gWire
}
