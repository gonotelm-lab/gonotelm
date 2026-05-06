package logic

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"

	bizchat "github.com/gonotelm-lab/gonotelm/internal/app/biz/chat"
	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type ChatLogic struct {
	bgWg        sync.WaitGroup
	notebookBiz *biznotebook.Biz
	sourceBiz   *bizsource.Biz
	chatBiz     *bizchat.Biz
}

func NewChatLogic(
	notebookBiz *biznotebook.Biz,
	sourceBiz *bizsource.Biz,
	chatBiz *bizchat.Biz,
) *ChatLogic {
	return &ChatLogic{
		notebookBiz: notebookBiz,
		sourceBiz:   sourceBiz,
		chatBiz:     chatBiz,
	}
}

func (l *ChatLogic) Close(ctx context.Context) {
	l.bgWg.Wait()
	slog.InfoContext(ctx, "chat logic closed")
}

type CreateUserMessageParams struct {
	NotebookId uuid.UUID
	Prompt     string
	SourceIds  []uuid.UUID
}

func (l *ChatLogic) CreateUserMessage(
	ctx context.Context,
	params *CreateUserMessageParams,
) (uuid.UUID, error) {
	// check notebook
	_, err := l.notebookBiz.GetNotebook(ctx, params.NotebookId)
	if err != nil {
		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
			return uuid.UUID{}, errors.ErrParams.Msgf("notebook not found, notebook_id=%s", params.NotebookId)
		}
		return uuid.UUID{}, errors.WithMessage(err, "get notebook failed")
	}

	// 粗略检查source ids是否存在且属于notebookid
	query := &bizsource.CheckSourceIdsQuery{
		NotebookId: params.NotebookId,
		SourceIds:  params.SourceIds,
	}
	existSourceIds, err := l.sourceBiz.CheckSourceIds(ctx, query)
	if err != nil {
		return uuid.UUID{}, errors.WithMessage(err, "check source ids failed")
	}
	if len(existSourceIds) == 0 {
		return uuid.UUID{}, errors.ErrParams.Msgf(
			"no source ids found, notebook_id=%s, source_ids=%v",
			params.NotebookId, params.SourceIds)
	}

	msgId, err := l.chatBiz.AddUserMessage(ctx, &bizchat.AddUserMessageCommand{
		ChatId:  params.NotebookId,
		UserId:  pkgcontext.GetUserId(ctx),
		Content: params.Prompt,
	})
	if err != nil {
		return uuid.UUID{}, errors.WithMessage(err, "add user message failed")
	}

	// 启动后台处理流程
	l.bgWg.Add(1)
	ctx = context.WithoutCancel(ctx)
	go l.processUserMessageTask(ctx, params.NotebookId, msgId, params)

	return msgId, nil
}

type RetrieveSourceDocsParams struct {
	NotebookId uuid.UUID
	Prompt     string
	SourceIds  []uuid.UUID
}

func (l *ChatLogic) RetrieveSourceDocs(
	ctx context.Context,
	params *RetrieveSourceDocsParams,
) ([]*model.SourceDoc, error) {
	query := &bizsource.CheckSourceIdsQuery{
		NotebookId: params.NotebookId,
		SourceIds:  params.SourceIds,
	}
	existSourceIds, err := l.sourceBiz.CheckSourceIds(ctx, query)
	if err != nil {
		return nil, errors.WithMessage(err, "check source ids failed")
	}
	if len(existSourceIds) == 0 {
		return nil, errors.ErrParams.Msgf(
			"no source ids found, notebook_id=%s, source_ids=%v",
			params.NotebookId, params.SourceIds)
	}

	retrieved, err := l.sourceBiz.RetrieveSourceDocs(ctx,
		&bizsource.RetrieveSourceDocsQuery{
			NotebookId: params.NotebookId,
			SourceIds:  existSourceIds,
			Query:      params.Prompt,
			Count:      conf.Global().Logic.Chat.SourceDocsRecallCount,
		})
	if err != nil {
		return nil, errors.WithMessage(err, "recall source docs failed")
	}

	slog.DebugContext(ctx, fmt.Sprintf("successfully recalled %d source docs", len(retrieved)))

	return retrieved, nil
}

func (l *ChatLogic) processUserMessageTask(
	ctx context.Context,
	chatId uuid.UUID,
	msgId uuid.UUID,
	params *CreateUserMessageParams,
) {
	defer func() {
		l.bgWg.Done()

		if e := recover(); e != nil {
			stacks := debug.Stack()
			slog.ErrorContext(ctx, "background process user message panic",
				slog.Any("err", e),
				slog.String("stack", string(stacks)),
			)
		}
	}()

	// get msg first
	_, err := l.chatBiz.GetMessage(ctx, msgId, chatId)
	if err != nil {
		// TODO 写入结果
		return
	}

	sourceDocs, err := l.RetrieveSourceDocs(ctx, &RetrieveSourceDocsParams{
		NotebookId: params.NotebookId,
		Prompt:     params.Prompt,
		SourceIds:  params.SourceIds,
	})
	if err != nil {
		// TODO 写入结果中
		return
	}

	if len(sourceDocs) == 0 {
		// TODO 写入结果
		return
	}
}
