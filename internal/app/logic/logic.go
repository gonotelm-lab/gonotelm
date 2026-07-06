package logic

import (
	"context"
	"sync"

	bizartifact "github.com/gonotelm-lab/gonotelm/internal/app/biz/artifact"
	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	sourcelogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/source"
	studiologic "github.com/gonotelm-lab/gonotelm/internal/app/logic/studio"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	dal "github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	llm "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/openai"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/vectordb"
	"github.com/redis/go-redis/v9"
)

type Logic struct {
	SourceLogic *sourcelogic.Logic
	StudioLogic *studiologic.Logic
}

func MustNewLogic(
	ctx context.Context,
	objectStorage storage.Storage,
	notebookStore dal.NotebookStore,
	artifactTaskStore dal.ArtifactTaskStore,
	sourceStore dal.SourceStore,
	sourceDocStore vectordb.SourceDocStore,
	llmGateway *openai.Gateway,
	embeddingGateway *llm.EmbeddingGateway,
	text2imageGateway *llm.Text2ImageGateway,
	mqFactory *mq.MQ,
	redisClient redis.UniversalClient,
) *Logic {
	notebookBiz := biznotebook.New(notebookStore)
	artifactBiz := bizartifact.New(artifactTaskStore)

	prompt := bizprompt.New("zh")

	sourceBiz, err := bizsource.New(
		objectStorage,
		sourceStore,
		sourceDocStore,
		llmGateway,
		embeddingGateway,
		prompt,
	)
	if err != nil {
		panic(err)
	}

	agentSourceBiz, err := bizsource.NewAgentBiz(ctx,
		sourceBiz,
		bizsource.AgentBizConfig{
			SourceCacheEviction: conf.Global().Logic.Source.BizCache.Eviction,
			SourceCacheMaxMB:    conf.Global().Logic.Source.BizCache.MaxMB,
		})
	if err != nil {
		panic(err)
	}

	sourceLogic := sourcelogic.MustNewLogic(
		ctx,
		mqFactory,
		redisClient,
		objectStorage,
		notebookBiz,
		sourceBiz,
		llmGateway,
		prompt,
	)

	studioLogic := studiologic.MustNewLogic(
		ctx,
		objectStorage,
		sourceBiz,
		agentSourceBiz,
		notebookBiz,
		artifactBiz,
		llmGateway,
		text2imageGateway,
		prompt,
	)

	return &Logic{
		SourceLogic: sourceLogic,
		StudioLogic: studioLogic,
	}
}

func (l *Logic) Close(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		l.SourceLogic.Close(ctx)
	})
	wg.Go(func() {
		l.StudioLogic.Close(ctx)
	})

	wg.Wait()
}
