package logic

import (
	"context"
	"sync"

	bizchat "github.com/gonotelm-lab/gonotelm/internal/app/biz/chat"
	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	chatlogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/chat"
	sourcelogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/source"
	studiologic "github.com/gonotelm-lab/gonotelm/internal/app/logic/studio"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
)

type Logic struct {
	NotebookLogic *NotebookLogic
	SourceLogic   *sourcelogic.Logic
	ChatLogic     *chatlogic.Logic
	StudioLogic   *studiologic.Logic
}

func MustNewLogic(
	ctx context.Context,
	infrastructures *infra.Instances,
	objectStorage storage.Storage,
) *Logic {
	// biz instances initialization
	var (
		notebookBiz = biznotebook.New(infrastructures.Dal.NotebookStore)
		chatBiz     = bizchat.New(
			infrastructures.Dal.ChatStore,
			infrastructures.Dal.ChatMessageStore,
			infrastructures.Cache.ChatMessageContextCache)
		chatEventManager = bizchat.NewChatEventManager(infrastructures.Cache.ChatMessageStreamCache)
	)

	gateway, err := gateway.New(&conf.Global().Provider)
	if err != nil {
		panic(err)
	}

	sourceBiz, err := bizsource.New(
		objectStorage,
		infrastructures.Dal.SourceStore,
		infrastructures.VectorDal.SourceDocStore,
		gateway,
	)
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

	notebookLogic := NewNotebookLogic(
		notebookBiz,
		sourceBiz,
		chatBiz,
	)

	chatLogic := chatlogic.MustNewLogic(
		gateway,
		notebookBiz,
		sourceBiz,
		chatBiz,
		chatEventManager,
	)

	studioLogic := studiologic.NewLogic(
		objectStorage,
		sourceBiz,
		notebookBiz,
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

	wg.Wait()
}
