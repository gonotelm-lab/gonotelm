package bootstrap

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/conf"
	oldcache "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	oldmqimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	oldstorageimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/adapter"
	cacheredis "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/redis"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/event"
	dbpostgres "github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/postgres"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	infrallm "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/openai"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	mqkafka "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq/kafka"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	storageminio "github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage/minio"
	vdbmilvus "github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/milvus"
)

type App struct {
	closers []io.Closer
	Server  interface{ Run() }
}

func (a *App) Close() error {
	for i := len(a.closers) - 1; i >= 0; i-- {
		if err := a.closers[i].Close(); err != nil {
			slog.Error("close error", "err", err)
		}
	}
	return nil
}

func NewApp(ctx context.Context, cfg *conf.Config) (_ *App, outErr error) {
	var closers []io.Closer
	addCloser := func(c io.Closer) { closers = append(closers, c) }
	// close all previously added closers on error
	defer func() {
		if outErr != nil {
			for i := len(closers) - 1; i >= 0; i-- {
				if err := closers[i].Close(); err != nil {
					slog.Error("close error", "err", err)
				}
			}
		}
	}()

	// ── 1. Infrastructure ──

	db, err := dbpostgres.Open(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}
	addCloser(contextCloser(func(ctx context.Context) error { return db.Close(ctx) }))

	vdb, err := vdbmilvus.Open(&cfg.VectorDB)
	if err != nil {
		return nil, fmt.Errorf("vectordb: %w", err)
	}
	addCloser(contextCloser(func(ctx context.Context) error { return vdb.Close(ctx) }))

	if err := oldcache.Init(&cfg.Redis); err != nil {
		return nil, fmt.Errorf("cache init: %w", err)
	}
	redisClient := oldcache.GetRedis()
	addCloser(contextCloser(func(ctx context.Context) error { return redisClient.Close() }))
	cacheInst := cacheredis.NewCache(redisClient)

	mqInst, err := newMQ(&cfg.MsgQueue)
	if err != nil {
		return nil, fmt.Errorf("mq: %w", err)
	}

	oss, err := newStorage(&cfg.Storage)
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}

	llmGateway, err := newLLMGateway(&cfg.Provider)
	if err != nil {
		return nil, fmt.Errorf("llm gateway: %w", err)
	}

	embeddingGateway, err := infrallm.NewEmbeddingGateway(
		&cfg.Embedding,
		infrallm.NewRedisCacher(redisClient),
	)
	if err != nil {
		return nil, fmt.Errorf("embedding gateway: %w", err)
	}

	embedder, err := embeddingGateway.GetProvider(cfg.Embedding.Type)
	if err != nil {
		return nil, fmt.Errorf("embedder: %w", err)
	}

	text2imageGateway, err := infrallm.NewText2ImageGateway(&cfg.Text2Image)
	if err != nil {
		return nil, fmt.Errorf("text2image gateway: %w", err)
	}

	// ── 2. Repositories ──

	notebookRepo := repository.NewNotebookRepository(db.NotebookStore, db.SourceStore)
	sourceRepo := repository.NewSourceRepository(db.SourceStore)
	sourceStorageRepo := repository.NewSourceStorageRepository(oss)
	sourceDocRepo := repository.NewSourceDocRepository(
		embedder,
		vdb.SourceDocStore,
		repository.SourceDocRepositoryConfig{
			EmbedBatchSize:      cfg.Embedding.BatchSize,
			EmbedMaxConcurrency: cfg.Embedding.MaxConcurrency,
		},
	)
	chatRepo := repository.NewChatRepository(db.ChatStore)
	messageRepo := repository.NewMessageRepository(db.ChatMessageStore)
	contextMsgRepo := repository.NewContextMessageRepository(cacheInst.ChatMessageContextCache)
	streamTaskRepo := repository.NewStreamTaskRepository(cacheInst.ChatMessageStreamCache)
	artifactTaskRepo := repository.NewArtifactTaskRepository(db.ArtifactTaskStore)

	// ── 3. Event Bus ──

	innerBus := eventbus.NewInnerEventBus()
	outerBus := eventbus.NewOuterEventBus(mqInst)
	bus := eventbus.NewCompositeEventBus(innerBus, outerBus)

	// ── 4. Adapters ──

	summarizer := adapter.NewSummarizer(llmGateway)

	// ── 5. Biz objects ──
	// TODO: Migrate biz constructors to accept database.* (NEW) types instead of dal.* (OLD) types.

	_ = text2imageGateway

	// ── 6. Logic ──
	// TODO: Migrate biz constructors to accept database.* (NEW) types instead of dal.* (OLD) types.
	/*
	appLogic := logic.MustNewLogic(
		ctx,
		oss,
		db.NotebookStore,
		db.ArtifactTaskStore,
		db.SourceStore,
		vdb.SourceDocStore,
		llmGateway,
		embeddingGateway,
		text2imageGateway,
		mqInst,
		redisClient,
	)
	_ = appLogic
	*/

	// ── 7. Event handler registration ──

	event.Init(ctx, &event.EventDeps{
		NotebookRepo:        notebookRepo,
		SourceRepo:          sourceRepo,
		SourceStorageRepo:   sourceStorageRepo,
		SourceDocRepo:       sourceDocRepo,
		ChatRepo:            chatRepo,
		MessageRepo:         messageRepo,
		ContextMessageRepo:  contextMsgRepo,
		ArtifactTaskRepo:    artifactTaskRepo,
		EventBus:            bus,
		Summarizer:          summarizer,
	})

	// ── 8. HTTP Server ──
	// TODO: Update api.NewServer to accept explicit params instead of *infra.Instances + *wire.Wire.
	// See Tasks 9-12.

	_ = streamTaskRepo

	return &App{closers: closers, Server: &dummyServer{}}, nil
}

type dummyServer struct{}

func (d *dummyServer) Run() {}

// ── internal helpers ──

func newLLMGateway(cfg *infrallm.ProviderConfig) (*openai.Gateway, error) {
	llmCfg := &infrallm.ProviderConfig{
		OpenAI: infrallm.OpenAIChatConfig{
			ApiKey:           	cfg.OpenAI.ApiKey,
			Timeout:          	cfg.OpenAI.Timeout,
			BaseUrl:          	cfg.OpenAI.BaseUrl,
			Model:            	cfg.OpenAI.Model,
			MaxTokens:        	cfg.OpenAI.MaxTokens,
			Temperature:      	cfg.OpenAI.Temperature,
			TopP:             	cfg.OpenAI.TopP,
			PresencePenalty:  	cfg.OpenAI.PresencePenalty,
			Seed:             	cfg.OpenAI.Seed,
			FrequencyPenalty: 	cfg.OpenAI.FrequencyPenalty,
			ReasoningEffort:  	cfg.OpenAI.ReasoningEffort,
			MaxConcurrency:   	cfg.OpenAI.MaxConcurrency,
		},
		DeepSeek: infrallm.DeepSeekChatConfig{
			ApiKey:           cfg.DeepSeek.ApiKey,
			Timeout:          cfg.DeepSeek.Timeout,
			BaseURL:          cfg.DeepSeek.BaseURL,
			Path:             cfg.DeepSeek.Path,
			Model:            cfg.DeepSeek.Model,
			MaxTokens:        cfg.DeepSeek.MaxTokens,
			Temperature:      cfg.DeepSeek.Temperature,
			TopP:             cfg.DeepSeek.TopP,
			PresencePenalty:  cfg.DeepSeek.PresencePenalty,
			FrequencyPenalty: cfg.DeepSeek.FrequencyPenalty,
			LogProbs:         cfg.DeepSeek.LogProbs,
			TopLogProbs:      cfg.DeepSeek.TopLogProbs,
			ThinkingEnabled:  cfg.DeepSeek.ThinkingEnabled,
			MaxConcurrency:   cfg.DeepSeek.MaxConcurrency,
		},
		Qwen: infrallm.QwenChatConfig{
			ApiKey:           cfg.Qwen.ApiKey,
			Timeout:          cfg.Qwen.Timeout,
			BaseUrl:          cfg.Qwen.BaseUrl,
			Model:            cfg.Qwen.Model,
			MaxTokens:        cfg.Qwen.MaxTokens,
			Temperature:      cfg.Qwen.Temperature,
			TopP:             cfg.Qwen.TopP,
			PresencePenalty:  cfg.Qwen.PresencePenalty,
			Seed:             cfg.Qwen.Seed,
			FrequencyPenalty: cfg.Qwen.FrequencyPenalty,
			EnableThinking:   cfg.Qwen.EnableThinking,
			MaxConcurrency:   cfg.Qwen.MaxConcurrency,
		},
		Agnes: infrallm.AgnesChatConfig{
			ApiKey:           cfg.Agnes.ApiKey,
			Timeout:          cfg.Agnes.Timeout,
			BaseUrl:          cfg.Agnes.BaseUrl,
			Model:            cfg.Agnes.Model,
			MaxTokens:        cfg.Agnes.MaxTokens,
			Temperature:      cfg.Agnes.Temperature,
			TopP:             cfg.Agnes.TopP,
			PresencePenalty:  cfg.Agnes.PresencePenalty,
			Seed:             cfg.Agnes.Seed,
			FrequencyPenalty: cfg.Agnes.FrequencyPenalty,
			MaxConcurrency:   cfg.Agnes.MaxConcurrency,
		},
	}
	return openai.New(llmCfg)
}

func newMQ(cfg *oldmqimpl.Config) (*mq.MQ, error) {
	switch cfg.Type {
	case oldmqimpl.Kafka:
		kc := cfg.Kafka
		return &mq.MQ{
			NewProducer: func() mq.Producer {
				return mqkafka.NewProducer(mqkafka.ProducerConfig{
					Brokers:  kc.Brokers,
					Username: kc.Username,
					Password: kc.Password,
				})
			},
			NewConsumer: func(topic, groupID string) mq.Consumer {
				return mqkafka.NewConsumer(mqkafka.ConsumerConfig{
					Brokers:        kc.Brokers,
					GroupID:        groupID,
					Topic:          topic,
					QueueCapacity:  kc.ConsumerQueueCapacity,
					CommitInterval: kc.ConsumerCommitInterval,
					Username:       kc.Username,
					Password:       kc.Password,
				})
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown mq type: %s", cfg.Type)
	}
}

func newStorage(cfg *oldstorageimpl.StorageTypeConfig) (storage.Storage, error) {
	switch cfg.Type {
	case oldstorageimpl.Minio:
		mc := cfg.Minio
		return storageminio.New(&storage.Config{
			Endpoint:      mc.Endpoint,
			Region:        mc.Region,
			Bucket:        mc.Bucket,
			AccessKey:     mc.AccessKey,
			SecretKey:     mc.SecretKey,
			Secure:        mc.Secure,
			PresignExpiry: mc.PresignExpiry,
		})
	default:
		return nil, fmt.Errorf("unknown storage type: %s", cfg.Type)
	}
}

type contextCloser func(ctx context.Context) error

func (c contextCloser) Close() error {
	return c(context.Background())
}
