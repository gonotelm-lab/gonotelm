package shared

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	confshared "github.com/gonotelm-lab/gonotelm/internal/conf/shared"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	infraadapter "github.com/gonotelm-lab/gonotelm/internal/infrastructure/adapter"
	infracache "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/redis"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/postgres"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/embedding"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	mqkafka "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq/kafka"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap/clickhouse"
	infrasandbox "github.com/gonotelm-lab/gonotelm/internal/infrastructure/sandbox"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage/minio"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/milvus"

	embedcache "github.com/cloudwego/eino-ext/components/embedding/cache"
	einoembed "github.com/cloudwego/eino/components/embedding"
	redisv9 "github.com/redis/go-redis/v9"
)

type Infra struct {
	Database         *database.Dao
	OlapDatabase     *olap.Dao
	VectorDatabase   *vectordb.DAL
	Cache            *infracache.Cache
	MessageQueue     *mq.MessageQueue
	Storage          storage.Storage
	LLMGateway       *llmchat.Gateway
	EmbeddingGateway *embedding.EmbeddingGateway
	Embedder         einoembed.Embedder
	Text2Image       *text2image.Text2ImageGateway
	Text2Audio       *text2audio.Text2AudioGateway
	SandboxGateway   *infrasandbox.Gateway
	DistLock         adapter.DistributedLock

	Redis redisv9.UniversalClient

	closers []io.Closer
}

func (i *Infra) addCloser(closer io.Closer) {
	i.closers = append(i.closers, closer)
}

func (i *Infra) Closers() []io.Closer { return i.closers }

func initDatabase(cfg *confshared.InfraConfig, infra *Infra) error {
	db, err := postgres.Open(cfg.Database.ToSQLConfig())
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}

	infra.addCloser(contextCloser(func(ctx context.Context) error {
		return db.Close(ctx)
	}))
	infra.Database = db

	return nil
}

func initOlapDatabase(cfg *confshared.InfraConfig, infra *Infra) error {
	dao, err := clickhouse.Open(context.Background(), cfg.DatabaseOlap.ToSQLConfig())
	if err != nil {
		return fmt.Errorf("olap database: %w", err)
	}

	infra.addCloser(contextCloser(func(ctx context.Context) error {
		return dao.Closer.Close(ctx)
	}))
	infra.OlapDatabase = dao

	return nil
}

func initVectorDB(cfg *confshared.InfraConfig, infra *Infra) error {
	vdb, err := milvus.Open(&cfg.VectorDB)
	if err != nil {
		return fmt.Errorf("vectordb: %w", err)
	}
	infra.addCloser(contextCloser(func(ctx context.Context) error {
		return vdb.Close(ctx)
	}))
	infra.VectorDatabase = vdb

	return nil
}

func initRedis(cfg *confshared.InfraConfig, infra *Infra) error {
	if len(cfg.Redis.Addrs) == 0 {
		return nil
	}

	if err := infracache.Init(&cfg.Redis); err != nil {
		return fmt.Errorf("cache init: %w", err)
	}
	redisClient := infracache.GetRedis()
	infra.addCloser(contextCloser(func(ctx context.Context) error {
		return redisClient.Close()
	}))
	infra.Redis = redisClient
	infra.Cache = redis.NewCache(redisClient)
	infra.DistLock = infraadapter.NewRedisDistributedLock(redisClient)

	return nil
}

func initMQ(cfg *confshared.InfraConfig, infra *Infra) error {
	if cfg.MsgQueue.Type == "" {
		return nil
	}

	mqInst, err := newMQ(&cfg.MsgQueue)
	if err != nil {
		return fmt.Errorf("mq: %w", err)
	}
	infra.MessageQueue = mqInst

	return nil
}

func initStorage(cfg *confshared.InfraConfig, infra *Infra) error {
	oss, err := newStorage(&cfg.Storage)
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	infra.Storage = oss

	return nil
}

func initLLMGateway(ctx context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
	// create llm recorder
	recorder := infraadapter.NewLLMRecorderAdapter(infra.OlapDatabase.LLMLogStore)
	llmGateway, err := llmchat.New(ctx, &cfg.Provider, llmchat.WithRecorder(recorder))
	if err != nil {
		return fmt.Errorf("llm gateway: %w", err)
	}
	infra.LLMGateway = llmGateway

	return nil
}

func initEmbedding(cfg *confshared.InfraConfig, infra *Infra) error {
	var embedCacher embedcache.Cacher
	if infra.Redis != nil {
		embedCacher = embedding.NewRedisCacher(infra.Redis)
	}
	recorder := infraadapter.NewEmbeddingRecorderAdapter(infra.OlapDatabase.EmbeddingLogStore)
	embeddingGateway, err := embedding.NewEmbeddingGateway(
		&cfg.Embedding,
		embedCacher,
		embedding.WithRecorder(recorder),
	)
	if err != nil {
		return fmt.Errorf("embedding gateway: %w", err)
	}
	infra.EmbeddingGateway = embeddingGateway

	embedder, err := embeddingGateway.GetProvider(cfg.Embedding.Type)
	if err != nil {
		return fmt.Errorf("embedder: %w", err)
	}
	infra.Embedder = embedder

	return nil
}

func initText2Image(cfg *confshared.InfraConfig, infra *Infra) error {
	text2imageGateway, err := text2image.NewText2ImageGateway(&cfg.Text2Image)
	if err != nil {
		return fmt.Errorf("text2image gateway: %w", err)
	}
	infra.Text2Image = text2imageGateway

	return nil
}

func initText2Audio(cfg *confshared.InfraConfig, infra *Infra) error {
	text2audioGateway, err := text2audio.NewText2AudioGateway(&cfg.Text2Audio)
	if err != nil {
		return fmt.Errorf("text2audio gateway: %w", err)
	}
	infra.Text2Audio = text2audioGateway

	return nil
}

func initSandbox(ctx context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
	sandboxGateway, err := infrasandbox.NewGateway(ctx, &cfg.Sandbox)
	if err != nil {
		return fmt.Errorf("sandbox gateway: %w", err)
	}
	infra.SandboxGateway = sandboxGateway

	return nil
}

func NewInfra(ctx context.Context, cfg *confshared.InfraConfig) (_ *Infra, finalErr error) {
	infra := &Infra{}
	defer func() {
		if finalErr != nil { // 初始化过程出错 就以此关闭已经初始化的组件
			for i := len(infra.closers) - 1; i >= 0; i-- {
				if err := infra.closers[i].Close(); err != nil {
					slog.Error("close error", "err", err)
				}
			}
		}
	}()

	// Do not switch the orders of initialization
	if err := initDatabase(cfg, infra); err != nil {
		return nil, err
	}
	if err := initOlapDatabase(cfg, infra); err != nil {
		return nil, err
	}
	if err := initVectorDB(cfg, infra); err != nil {
		return nil, err
	}
	if err := initRedis(cfg, infra); err != nil {
		return nil, err
	}
	if err := initMQ(cfg, infra); err != nil {
		return nil, err
	}
	if err := initStorage(cfg, infra); err != nil {
		return nil, err
	}
	if err := initLLMGateway(ctx, cfg, infra); err != nil {
		return nil, err
	}
	if err := initEmbedding(cfg, infra); err != nil {
		return nil, err
	}
	if err := initText2Image(cfg, infra); err != nil {
		return nil, err
	}
	if err := initText2Audio(cfg, infra); err != nil {
		return nil, err
	}
	if err := initSandbox(ctx, cfg, infra); err != nil {
		return nil, err
	}

	return infra, nil
}

func (s *Infra) Close(ctx context.Context) error {
	// in reverse order
	for i := len(s.closers) - 1; i >= 0; i-- {
		if err := s.closers[i].Close(); err != nil {
			slog.Error("close error", "err", err)
		}
	}
	return nil
}

func newMQ(cfg *mq.Config) (*mq.MessageQueue, error) {
	switch cfg.Type {
	case mq.Kafka:
		kc := cfg.Kafka
		return &mq.MessageQueue{
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

func newStorage(cfg *storage.StorageTypeConfig) (storage.Storage, error) {
	switch cfg.Type {
	case storage.Minio:
		mc := cfg.Minio
		return minio.New(&storage.Config{
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
