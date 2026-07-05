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
	"github.com/gonotelm-lab/gonotelm/internal/infra"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/embedding"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/text2image"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
)

type Logic struct {
	SourceLogic *sourcelogic.Logic
	StudioLogic *studiologic.Logic
}

func MustNewLogic(
	ctx context.Context,
	infrastructures *infra.Instances,
	objectStorage storage.Storage,
) *Logic {
	llmGateway, err := gateway.New(&conf.Global().Provider)
	if err != nil {
		panic(err)
	}

	embeddingGateway, err := embedding.NewGateway(
		&conf.Global().Embedding,
		embedding.NewRedisCacher(cache.GetRedis()),
	)
	if err != nil {
		panic(err)
	}

	text2imageGateway, err := text2image.NewGateway(
		&conf.Global().Text2Image,
	)
	if err != nil {
		panic(err)
	}

	notebookBiz := biznotebook.New(infrastructures.Dal.NotebookStore)
	artifactBiz := bizartifact.New(infrastructures.Dal.ArtifactTaskStore)

	prompt := bizprompt.New("zh")

	sourceBiz, err := bizsource.New(
		objectStorage,
		infrastructures.Dal.SourceStore,
		infrastructures.VectorDal.SourceDocStore,
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
		infrastructures,
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
