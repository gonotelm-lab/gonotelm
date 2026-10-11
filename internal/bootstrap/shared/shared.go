package shared

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	confshared "github.com/gonotelm-lab/gonotelm/internal/conf/shared"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	infraadapter "github.com/gonotelm-lab/gonotelm/internal/infrastructure/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/redis"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/postgres"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	llmchatbilling "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat/billing"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/embedding"
	embbilling "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/embedding/billing"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	t2abilling "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio/billing"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image"
	t2ibilling "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2image/billing"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	mqkafka "github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq/kafka"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap/clickhouse"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/sandbox"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage/minio"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb/milvus"

	embedcache "github.com/cloudwego/eino-ext/components/embedding/cache"
	einoembed "github.com/cloudwego/eino/components/embedding"
	redisv9 "github.com/redis/go-redis/v9"
)

type Infra struct {
	Database               *database.Dao
	OlapDatabase           *olap.Dao
	VectorDatabase         *vectordb.DAL
	Cache                  *cache.Cache
	MessageQueue           *mq.MessageQueue
	Storage                storage.Storage
	PublicStorage          storage.PublicReadStorage
	ObjectStore            adapter.ObjectStore
	KeyFactory             adapter.StoreKeyFactory
	LLMGateway             *llmchat.Gateway
	LLMBillingMeter        llmchatbilling.Meter
	EmbeddingGateway       *embedding.EmbeddingGateway
	EmbeddingBillingMeter  embbilling.Meter
	Text2ImageBillingMeter t2ibilling.Meter
	Text2AudioBillingMeter t2abilling.Meter
	Embedder               einoembed.Embedder
	Text2Image             *text2image.Text2ImageGateway
	Text2Audio             *text2audio.Text2AudioGateway
	SandboxGateway         *sandbox.Gateway
	DistLock               adapter.DistributedLock
	Redis                  redisv9.UniversalClient

	closers []io.Closer
}

func (i *Infra) addCloser(closer io.Closer) {
	i.closers = append(i.closers, closer)
}

func (i *Infra) Closers() []io.Closer { return i.closers }

func initDatabase(_ context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
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

func initOlapDatabase(_ context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
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

func initVectorDB(_ context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
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

func initRedis(_ context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
	if len(cfg.Redis.Addrs) == 0 {
		return nil
	}

	if err := cache.Init(&cfg.Redis); err != nil {
		return fmt.Errorf("cache init: %w", err)
	}
	redisClient := cache.GetRedis()
	infra.addCloser(contextCloser(func(ctx context.Context) error {
		return redisClient.Close()
	}))
	infra.Redis = redisClient
	infra.Cache = redis.NewCache(redisClient)
	infra.DistLock = infraadapter.NewRedisDistributedLock(redisClient)

	return nil
}

func initMessageQueue(_ context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
	if cfg.MessageQueue.Type == "" {
		return nil
	}

	mqInst, err := newMessageQueue(&cfg.MessageQueue)
	if err != nil {
		return fmt.Errorf("mq: %w", err)
	}
	infra.MessageQueue = mqInst

	return nil
}

func initStorage(_ context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
	privateStorage, err := newStorage(&cfg.Storage)
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	infra.Storage = privateStorage

	publicStorage, err := newPublicReadStorage(&cfg.Storage)
	if err != nil {
		return fmt.Errorf("public read storage: %w", err)
	}
	infra.PublicStorage = publicStorage

	infra.ObjectStore = infraadapter.NewObjectStore(privateStorage, publicStorage)
	infra.KeyFactory = infraadapter.NewStoreKeyFactory(privateStorage, publicStorage)

	return nil
}

func initLLMGateway(ctx context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
	// create llm recorder
	recorder := infraadapter.NewLLMRecorderAdapter(
		infra.OlapDatabase.LLMLogStore,
		infra.LLMBillingMeter,
	)
	llmGateway, err := llmchat.New(ctx, &cfg.Provider,
		llmchat.WithRecorder(recorder),
		llmchat.WithRecordTruncate(cfg.LLMRecord.TruncateEnabled()),
	)
	if err != nil {
		return fmt.Errorf("llm gateway: %w", err)
	}
	infra.LLMGateway = llmGateway

	return nil
}

func initLLMBillingMeters(_ context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
	meter, err := llmchatbilling.NewStandardMeter(llmchatbilling.StandardMeterConfig{
		DeepSeekScript: cfg.ProviderBilling.DeepSeekScript,
		QwenScript:     cfg.ProviderBilling.ChatQwenScript,
	})
	if err != nil {
		return fmt.Errorf("llm chat billing: %w", err)
	}
	infra.LLMBillingMeter = meter

	return nil
}

func initEmbeddingBillingMeter(_ context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
	meter, err := embbilling.NewStandardMeter(embbilling.StandardMeterConfig{
		QwenScript: cfg.ProviderBilling.EmbeddingQwenScript,
	})
	if err != nil {
		return fmt.Errorf("llm embedding billing: %w", err)
	}
	infra.EmbeddingBillingMeter = meter

	return nil
}

func initText2ImageBillingMeter(_ context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
	meter, err := t2ibilling.NewStandardMeter(t2ibilling.StandardMeterConfig{
		QwenScript: cfg.ProviderBilling.Text2ImageQwenScript,
	})
	if err != nil {
		return fmt.Errorf("llm text2image billing: %w", err)
	}
	infra.Text2ImageBillingMeter = meter

	return nil
}

func initText2AudioBillingMeter(_ context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
	meter, err := t2abilling.NewStandardMeter(t2abilling.StandardMeterConfig{
		QwenScript:    cfg.ProviderBilling.Text2AudioQwenScript,
		MiniMaxScript: cfg.ProviderBilling.Text2AudioMiniMaxScript,
	})
	if err != nil {
		return fmt.Errorf("llm text2audio billing: %w", err)
	}
	infra.Text2AudioBillingMeter = meter

	return nil
}

func initEmbedding(_ context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
	var embedCacher embedcache.Cacher
	if infra.Redis != nil {
		embedCacher = embedding.NewRedisCacher(infra.Redis)
	}
	recorder := infraadapter.NewEmbeddingRecorderAdapter(infra.OlapDatabase.EmbeddingLogStore, infra.EmbeddingBillingMeter)
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

func initText2Image(_ context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
	recorder := infraadapter.NewText2ImageRecorderAdapter(
		infra.OlapDatabase.Text2ImageLogStore,
		infra.Text2ImageBillingMeter,
	)
	text2imageGateway, err := text2image.NewText2ImageGateway(
		&cfg.Text2Image,
		text2image.WithRecorder(recorder),
	)
	if err != nil {
		return fmt.Errorf("text2image gateway: %w", err)
	}
	infra.Text2Image = text2imageGateway

	return nil
}

func initText2Audio(_ context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
	recorder := infraadapter.NewText2AudioRecorderAdapter(
		infra.OlapDatabase.Text2AudioLogStore,
		infra.Text2AudioBillingMeter,
	)
	text2audioGateway, err := text2audio.NewText2AudioGateway(
		&cfg.Text2Audio,
		text2audio.WithRecorder(recorder),
	)
	if err != nil {
		return fmt.Errorf("text2audio gateway: %w", err)
	}
	infra.Text2Audio = text2audioGateway

	return nil
}

func initSandbox(ctx context.Context, cfg *confshared.InfraConfig, infra *Infra) error {
	sandboxGateway, err := sandbox.NewGateway(ctx, &cfg.Sandbox)
	if err != nil {
		return fmt.Errorf("sandbox gateway: %w", err)
	}
	infra.SandboxGateway = sandboxGateway

	return nil
}

// Component names one infra component that NewInfraWith can build.
type Component string

const (
	ComponentDatabase          Component = "database"
	ComponentOlap              Component = "olap"
	ComponentVectorDB          Component = "vectorDb"
	ComponentRedis             Component = "redis"
	ComponentMQ                Component = "mq"
	ComponentStorage           Component = "storage"
	ComponentLLMBilling        Component = "llmBilling"
	ComponentEmbeddingBilling  Component = "embeddingBilling"
	ComponentText2ImageBilling Component = "text2imageBilling"
	ComponentText2AudioBilling Component = "text2audioBilling"
	ComponentLLMGateway        Component = "llmGateway"
	ComponentEmbedding         Component = "embedding"
	ComponentText2Image        Component = "text2image"
	ComponentText2Audio        Component = "text2audio"
	ComponentSandbox           Component = "sandbox"
)

type infraInit struct {
	name Component
	fn   func(ctx context.Context, cfg *confshared.InfraConfig, infra *Infra) error
}

// infraInits returns every component in construction order.
// Do not switch the orders of initialization.
func infraInits() []infraInit {
	return []infraInit{
		{ComponentDatabase, initDatabase},
		{ComponentOlap, initOlapDatabase},
		{ComponentVectorDB, initVectorDB},
		{ComponentRedis, initRedis},
		{ComponentMQ, initMessageQueue},
		{ComponentStorage, initStorage},
		{ComponentLLMBilling, initLLMBillingMeters},
		{ComponentEmbeddingBilling, initEmbeddingBillingMeter},
		{ComponentText2ImageBilling, initText2ImageBillingMeter},
		{ComponentText2AudioBilling, initText2AudioBillingMeter},
		{ComponentLLMGateway, initLLMGateway},
		{ComponentEmbedding, initEmbedding},
		{ComponentText2Image, initText2Image},
		{ComponentText2Audio, initText2Audio},
		{ComponentSandbox, initSandbox},
	}
}

// componentRequires declares which components read another component's output,
// so a missing one fails with a clear message instead of a nil dereference.
var componentRequires = map[Component][]Component{
	ComponentLLMGateway: {ComponentOlap},
	ComponentEmbedding:  {ComponentOlap},
	ComponentText2Image: {ComponentOlap},
	ComponentText2Audio: {ComponentOlap},
}

// NewInfra builds the full infra stack, preserving the behaviour of every
// existing process. Jobs that need less should call NewInfraWith.
func NewInfra(ctx context.Context, cfg *confshared.InfraConfig) (*Infra, error) {
	inits := infraInits()
	components := make([]Component, 0, len(inits))
	for _, init := range inits {
		components = append(components, init.name)
	}
	return NewInfraWith(ctx, cfg, components...)
}

// NewInfraWith builds only the requested components, in the fixed construction
// order. Wiring belongs in code, not in config: the caller states what it needs
// and the compiler checks the call.
func NewInfraWith(
	ctx context.Context, cfg *confshared.InfraConfig, components ...Component,
) (_ *Infra, finalErr error) {
	if err := validateComponents(components); err != nil {
		return nil, err
	}

	enabled := componentSet(components)

	infra := &Infra{}
	defer func() {
		if finalErr != nil { // on failure, close whatever was already built
			for i := len(infra.closers) - 1; i >= 0; i-- {
				if err := infra.closers[i].Close(); err != nil {
					slog.Error("close error", "err", err)
				}
			}
		}
	}()

	for _, init := range infraInits() {
		if _, ok := enabled[init.name]; !ok {
			continue
		}
		if err := init.fn(ctx, cfg, infra); err != nil {
			return nil, err
		}
	}

	return infra, nil
}

func validateComponents(components []Component) error {
	known := make(map[Component]struct{})
	for _, init := range infraInits() {
		known[init.name] = struct{}{}
	}
	for _, component := range components {
		if _, ok := known[component]; !ok {
			return fmt.Errorf("infra: unknown component %q", component)
		}
	}

	enabled := componentSet(components)
	for _, init := range infraInits() {
		if _, ok := enabled[init.name]; !ok {
			continue
		}
		for _, dep := range componentRequires[init.name] {
			if _, ok := enabled[dep]; !ok {
				return fmt.Errorf("infra: component %q requires %q", init.name, dep)
			}
		}
	}

	return nil
}

func componentSet(components []Component) map[Component]struct{} {
	set := make(map[Component]struct{}, len(components))
	for _, component := range components {
		set[component] = struct{}{}
	}
	return set
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

func newMessageQueue(cfg *mq.Config) (*mq.MessageQueue, error) {
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

func newPublicReadStorage(cfg *storage.StorageTypeConfig) (storage.PublicReadStorage, error) {
	publicCfg, err := cfg.PublicObjectStorageConfig()
	if err != nil {
		return nil, err
	}
	if publicCfg == nil {
		return nil, nil
	}

	switch cfg.Type {
	case storage.Minio:
		return minio.New(publicCfg)
	default:
		return nil, fmt.Errorf("public read storage is not supported for type %s", cfg.Type)
	}
}

type contextCloser func(ctx context.Context) error

func (c contextCloser) Close() error {
	return c(context.Background())
}
