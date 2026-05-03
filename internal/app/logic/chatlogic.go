package logic

import (
	"context"

	bizchat "github.com/gonotelm-lab/gonotelm/internal/app/biz/chat"
	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type ChatLogic struct {
	notebookBiz    *biznotebook.Biz
	sourceBiz      *bizsource.Biz
	chatMessageBiz *bizchat.ChatMessageBiz
}

func NewChatLogic(
	notebookBiz *biznotebook.Biz,
	sourceBiz *bizsource.Biz,
	chatMessageBiz *bizchat.ChatMessageBiz,
) *ChatLogic {
	return &ChatLogic{
		notebookBiz:    notebookBiz,
		sourceBiz:      sourceBiz,
		chatMessageBiz: chatMessageBiz,
	}
}

type AskAccordingToSourcesParams struct {
	NotebookId uuid.UUID
	Prompt     string
	SourceIds  []uuid.UUID
}

func (l *ChatLogic) AskAccordingToSources(
	ctx context.Context,
	params *AskAccordingToSourcesParams,
) error {
	// check notebook
	_, err := l.notebookBiz.GetNotebook(ctx, params.NotebookId)
	if err != nil {
		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
			return errors.ErrParams.Msgf("notebook not found, notebook_id=%s", params.NotebookId)
		}
		return errors.WithMessage(err, "get notebook failed")
	}

	// 粗略检查source ids是否存在且属于notebookid
	query := &bizsource.CheckSourceIdsQuery{
		NotebookId: params.NotebookId,
		SourceIds:  params.SourceIds,
	}
	existSourceIds, err := l.sourceBiz.CheckSourceIds(ctx, query)
	if err != nil {
		return errors.WithMessage(err, "check source ids failed")
	}
	if len(existSourceIds) == 0 {
		return errors.ErrParams.Msgf(
			"no source ids found, notebook_id=%s, source_ids=%v",
			params.NotebookId, params.SourceIds)
	}

	// 处理问答逻辑

	return nil
}
