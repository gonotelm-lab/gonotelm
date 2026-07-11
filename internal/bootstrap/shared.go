package bootstrap

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/redis/go-redis/v9"

	einoembed "github.com/cloudwego/eino/components/embedding"

	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/agentize"
	oldcache "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	cacheredis "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/redis"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	dbpostgres "github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/postgres"
	infrallm "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	embedding "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/embedding"
	text2image "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	oldmqimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	mqkafka "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq/kafka"
	infrarepo "github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	oldstorageimpl "github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	storageminio "github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage/minio"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb"
	vdbmilvus "github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/milvus"
)

type SharedInfra struct {
	DB               *database.DAL
	VDB              *vectordb.DAL
	Redis            redis.UniversalClient
	Cache            *oldcache.Cache
	MQ               *mq.MQ
	Storage          storage.Storage
	LLMGateway       *chat.Gateway
	EmbeddingGateway *embedding.EmbeddingGateway
	Embedder         einoembed.Embedder
	Text2Image       *text2image.Text2ImageGateway
	AgentizeService  *agentize.Service

	closers []io.Closer
}

func (s *SharedInfra) Closers() []io.Closer { return s.closers }

func NewSharedInfra(ctx context.Context, cfg *conf.Config) (_ *SharedInfra, outErr error) {
	infra := &SharedInfra{}
	addCloser := func(c io.Closer) { infra.closers = append(infra.closers, c) }
	defer func() {
		if outErr != nil {
			for i := len(infra.closers) - 1; i >= 0; i-- {
				if err := infra.closers[i].Close(); err != nil {
					slog.Error("close error", "err", err)
				}
			}
		}
	}()

	db, err := dbpostgres.Open(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}
	addCloser(contextCloser(func(ctx context.Context) error { return db.Close(ctx) }))
	infra.DB = db

	vdb, err := vdbmilvus.Open(&cfg.VectorDB)
	if err != nil {
		return nil, fmt.Errorf("vectordb: %w", err)
	}
	addCloser(contextCloser(func(ctx context.Context) error { return vdb.Close(ctx) }))
	infra.VDB = vdb

	if err := oldcache.Init(&cfg.Redis); err != nil {
		return nil, fmt.Errorf("cache init: %w", err)
	}
	redisClient := oldcache.GetRedis()
	addCloser(contextCloser(func(ctx context.Context) error { return redisClient.Close() }))
	infra.Redis = redisClient
	infra.Cache = cacheredis.NewCache(redisClient)

	mqInst, err := newMQ(&cfg.MsgQueue)
	if err != nil {
		return nil, fmt.Errorf("mq: %w", err)
	}
	infra.MQ = mqInst

	oss, err := newStorage(&cfg.Storage)
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	infra.Storage = oss

	llmGateway, err := newLLMGateway(&cfg.Provider)
	if err != nil {
		return nil, fmt.Errorf("llm gateway: %w", err)
	}
	infra.LLMGateway = llmGateway

	embeddingGateway, err := embedding.NewEmbeddingGateway(
		&cfg.Embedding,
		embedding.NewRedisCacher(redisClient),
	)
	if err != nil {
		return nil, fmt.Errorf("embedding gateway: %w", err)
	}
	infra.EmbeddingGateway = embeddingGateway

	embedder, err := embeddingGateway.GetProvider(cfg.Embedding.Type)
	if err != nil {
		return nil, fmt.Errorf("embedder: %w", err)
	}
	infra.Embedder = embedder

	text2imageGateway, err := text2image.NewText2ImageGateway(&cfg.Text2Image)
	if err != nil {
		return nil, fmt.Errorf("text2image gateway: %w", err)
	}
	infra.Text2Image = text2imageGateway

	sourceRepo := infrarepo.NewSourceRepository(db.SourceStore)
	storageRepo := infrarepo.NewSourceStorageRepository(oss)
	sourceDocRepo := infrarepo.NewSourceDocRepository(
		embedder,
		vdb.SourceDocStore,
		infrarepo.SourceDocRepositoryConfig{
			EmbedBatchSize:      cfg.Embedding.BatchSize,
			EmbedMaxConcurrency: cfg.Embedding.MaxConcurrency,
		},
	)
	infra.AgentizeService = agentize.NewService(agentize.Config{}, sourceRepo, storageRepo, sourceDocRepo)

	return infra, nil
}

func (s *SharedInfra) Close(ctx context.Context) error {
	for i := len(s.closers) - 1; i >= 0; i-- {
		if err := s.closers[i].Close(); err != nil {
			slog.Error("close error", "err", err)
		}
	}
	return nil
}

func newLLMGateway(cfg *infrallm.ProviderConfig) (*chat.Gateway, error) {
	llmCfg := &infrallm.ProviderConfig{
		OpenAI: infrallm.OpenAIChatConfig{
			ApiKey:           cfg.OpenAI.ApiKey,
			Timeout:          cfg.OpenAI.Timeout,
			BaseUrl:          cfg.OpenAI.BaseUrl,
			Model:            cfg.OpenAI.Model,
			MaxTokens:        cfg.OpenAI.MaxTokens,
			Temperature:      cfg.OpenAI.Temperature,
			TopP:             cfg.OpenAI.TopP,
			PresencePenalty:  cfg.OpenAI.PresencePenalty,
			Seed:             cfg.OpenAI.Seed,
			FrequencyPenalty: cfg.OpenAI.FrequencyPenalty,
			ReasoningEffort:  cfg.OpenAI.ReasoningEffort,
			MaxConcurrency:   cfg.OpenAI.MaxConcurrency,
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
	return chat.New(llmCfg)
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
