package logic

import (
	"context"
	"sync"

	bizchat "github.com/gonotelm-lab/gonotelm/internal/app/biz/chat"
	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	chatlogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/chat"
	sourcelogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/source"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
)

type Logic struct {
	NotebookLogic *NotebookLogic
	SourceLogic   *sourcelogic.SourceLogic
	ChatLogic     *chatlogic.Logic
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

	sourceBiz, err := bizsource.New(
		objectStorage,
		infrastructures.Dal.SourceStore,
		infrastructures.VectorDal.SourceDocStore,
	)
	if err != nil {
		panic(err)
	}

	gateway, err := gateway.New(&conf.Global().Provider)
	if err != nil {
		panic(err)
	}

	notebookLogic := NewNotebookLogic(
		notebookBiz,
		sourceBiz,
		chatBiz,
	)

	sourceLogic := sourcelogic.MustNewSourceLogic(
		ctx,
		objectStorage,
		notebookLogic.notebookBiz,
		sourceBiz,
		gateway,
	)

	chatLogic := chatlogic.MustNewLogic(
		gateway,
		notebookBiz,
		sourceBiz,
		chatBiz,
		chatEventManager,
	)

	return &Logic{
		NotebookLogic: notebookLogic,
		SourceLogic:   sourceLogic,
		ChatLogic:     chatLogic,
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

