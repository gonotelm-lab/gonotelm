package logic

import (
	"context"
	"sync"

	bizartifact "github.com/gonotelm-lab/gonotelm/internal/app/biz/artifact"
	bizchat "github.com/gonotelm-lab/gonotelm/internal/app/biz/chat"
	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	chatlogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/chat"
	notebooklogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/notebook"
	sourcelogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/source"
	studiologic "github.com/gonotelm-lab/gonotelm/internal/app/logic/studio"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
)

type Logic struct {
	NotebookLogic *notebooklogic.Logic
	SourceLogic   *sourcelogic.Logic
	ChatLogic     *chatlogic.Logic
	StudioLogic   *studiologic.Logic
}

func MustNewLogic(
	ctx context.Context,
	infrastructures *infra.Instances,
	objectStorage storage.Storage,
) *Logic {
	gateway, err := gateway.New(&conf.Global().Provider)
	if err != nil {
		panic(err)
	}

	// biz instances initialization
	notebookBiz := biznotebook.New(infrastructures.Dal.NotebookStore)
	chatBiz := bizchat.New(
		infrastructures.Dal.ChatStore,
		infrastructures.Dal.ChatMessageStore,
		infrastructures.Cache.ChatMessageContextCache)
	artifactBiz := bizartifact.New(infrastructures.Dal.ArtifactTaskStore)
	chatEventManager := bizchat.NewChatEventManager(
		infrastructures.Cache.ChatMessageStreamCache)

	sourceBiz, err := bizsource.New(
		objectStorage,
		infrastructures.Dal.SourceStore,
		infrastructures.VectorDal.SourceDocStore,
		gateway,
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
		gateway,
	)

	notebookLogic := notebooklogic.NewLogic(
		notebookBiz,
		sourceBiz,
		chatBiz,
		artifactBiz,
	)

	chatLogic := chatlogic.MustNewLogic(
		gateway,
		notebookBiz,
		sourceBiz,
		agentSourceBiz,
		chatBiz,
		chatEventManager,
	)

	studioLogic := studiologic.MustNewLogic(
		ctx,
		objectStorage,
		sourceBiz,
		agentSourceBiz,
		notebookBiz,
		artifactBiz,
		gateway,
	)

	return &Logic{
		NotebookLogic: notebookLogic,
		SourceLogic:   sourceLogic,
		ChatLogic:     chatLogic,
		StudioLogic:   studioLogic,
	}
}

func (l *Logic) Close(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() {
		l.SourceLogic.Close(ctx)
	})
	wg.Go(func() {
		l.ChatLogic.Close(ctx)
	})
	wg.Go(func() {
		l.StudioLogic.Close(ctx)
	})

	wg.Wait()
}
