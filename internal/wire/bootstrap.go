package wire

import (
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	adapterdefine "github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infra"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/embedding"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	adapterimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository"
)

var gWire *Wire

type Wire struct {
	llmGateway   *gateway.Gateway
	embedGateway *embedding.Gateway

	NotebookRepo      notebookrepo.Repository
	SourceRepo        sourcerepo.Repository
	SourceStorageRepo sourcerepo.StorageRepository
	SourceDocRepo     sourcerepo.SourceDocRepository

	EventBus eventbus.EventBus

	Summarizer adapterdefine.Summarizer
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

	llmGateway, err := gateway.New(&conf.Global().Provider)
	if err != nil {
		panic(err)
	}

	summarizer := adapterimpl.NewSummarizer(llmGateway)

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
		llmGateway:   llmGateway,
		embedGateway: embedGateway,

		NotebookRepo:      notebookRepo,
		SourceRepo:        sourceRepo,
		SourceStorageRepo: sourceStorageRepo,
		SourceDocRepo:     sourceDocRepo,
		EventBus:          eventbus.NewOuterEventBus(infras.MQ),

		Summarizer: summarizer,
	}
}

func GetWire() *Wire {
	return gWire
}
