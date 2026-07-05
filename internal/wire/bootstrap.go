package wire

import (
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/conf"
	adapterdefine "github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
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

	ChatRepo           chatrepo.Repository
	MessageRepo        chatrepo.MessageRepository
	StreamTaskRepo     chatrepo.StreamTaskRepository
	ContextMessageRepo chatrepo.ContextMessageRepository
	ArtifactTaskRepo   *repository.ArtifactTaskRepository

	EventBus eventbus.EventBus

	WaitGroup *sync.WaitGroup

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
		},
	)
	chatRepo := repository.NewChatRepository(infras.Dal.ChatStore)
	messageRepo := repository.NewMessageRepository(infras.Dal.ChatMessageStore)
	streamTaskRepo := repository.NewStreamTaskRepository(infras.Cache.ChatMessageStreamCache)
	contextMessageRepo := repository.NewContextMessageRepository(infras.Cache.ChatMessageContextCache)
	artifactTaskRepo := repository.NewArtifactTaskRepository(infras.Dal.ArtifactTaskStore)

	innerEventBus := eventbus.NewInnerEventBus()
	outerEventBus := eventbus.NewOuterEventBus(infras.MQ)

	gWire = &Wire{
		WaitGroup:    &sync.WaitGroup{},
		llmGateway:   llmGateway,
		embedGateway: embedGateway,

		NotebookRepo:       notebookRepo,
		SourceRepo:         sourceRepo,
		SourceStorageRepo:  sourceStorageRepo,
		SourceDocRepo:      sourceDocRepo,
		ChatRepo:           chatRepo,
		MessageRepo:        messageRepo,
		StreamTaskRepo:     streamTaskRepo,
		ContextMessageRepo: contextMessageRepo,
		ArtifactTaskRepo:   artifactTaskRepo,
		EventBus:           eventbus.NewCompositeEventBus(innerEventBus, outerEventBus),

		Summarizer: summarizer,
	}
}

func GetWire() *Wire {
	return gWire
}

func (w *Wire) Gateway() *gateway.Gateway {
	return w.llmGateway
}

func (w *Wire) Close() {
	w.WaitGroup.Wait()
}

func (w *Wire) Go(f func()) {
	w.WaitGroup.Go(f)
}
