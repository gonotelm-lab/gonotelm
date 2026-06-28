package notebook

import (
	"context"
	"log/slog"

	bizartifact "github.com/gonotelm-lab/gonotelm/internal/app/biz/artifact"
	bizchat "github.com/gonotelm-lab/gonotelm/internal/app/biz/chat"
	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	chatmodel "github.com/gonotelm-lab/gonotelm/internal/app/model/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Logic struct {
	notebookBiz *biznotebook.Biz
	sourceBiz   *bizsource.Biz
	chatBiz     *bizchat.Biz
	artifactBiz *bizartifact.Biz
}

func NewLogic(
	notebookBiz *biznotebook.Biz,
	sourceBiz *bizsource.Biz,
	chatBiz *bizchat.Biz,
	artifactBiz *bizartifact.Biz,
) *Logic {
	return &Logic{
		notebookBiz: notebookBiz,
		sourceBiz:   sourceBiz,
		chatBiz:     chatBiz,
		artifactBiz: artifactBiz,
	}
}

type CreateNotebookParams struct {
	Name string
	Desc string
}

// func (l *Logic) CreateNotebook(
// 	ctx context.Context,
// 	params *CreateNotebookParams,
// ) (*model.Notebook, error) {
// 	userId := pkgcontext.GetUserId(ctx)
// 	notebook, err := l.notebookBiz.CreateNotebook(
// 		ctx, &biznotebook.CreateNotebookCommand{
// 			Name:        params.Name,
// 			OwnerId:     userId,
// 			Description: params.Desc,
// 		})
// 	if err != nil {
// 		return nil, errors.WithMessage(err, "create notebook failed")
// 	}

// 	return notebook, nil
// }

type NotebookSummary struct {
	Notebook    *model.Notebook
	SourceCount int64
}

// func (l *Logic) GetNotebook(
// 	ctx context.Context,
// 	id uuid.UUID,
// ) (*NotebookSummary, error) {
// 	notebook, err := l.notebookBiz.GetNotebook(ctx, id)
// 	if err != nil {
// 		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
// 			return nil, errors.ErrParams.Msgf("notebook not found, notebook_id=%s", id)
// 		}

// 		return nil, errors.WithMessage(err, "get notebook failed")
// 	}

// 	output, err := l.buildNotebookSummary(ctx, notebook)
// 	if err != nil {
// 		return nil, errors.WithMessage(err, "build notebook output failed")
// 	}

// 	return output, nil
// }

type ListNotebooksSortBy int

const (
	ListNotebooksSortByCreateTime ListNotebooksSortBy = 0
	ListNotebooksSortByLastActive ListNotebooksSortBy = 1
)

type ListNotebooksParams struct {
	Limit  int
	Offset int
	SortBy ListNotebooksSortBy
}

type ListNotebooksResult struct {
	Notebooks []*NotebookSummary
	HasMore   bool
}

func (l *Logic) ListNotebooks(
	ctx context.Context,
	params *ListNotebooksParams,
) (*ListNotebooksResult, error) {
	userId := pkgcontext.GetUserId(ctx)
	result, err := l.notebookBiz.ListNotebooks(
		ctx,
		&biznotebook.ListNotebooksQuery{
			Limit:   params.Limit,
			OwnerId: userId,
			Offset:  params.Offset,
			SortBy:  int(params.SortBy),
		},
	)
	if err != nil {
		return nil, errors.WithMessage(err, "list notebooks failed")
	}

	notebooks := make([]*NotebookSummary, 0, len(result.Notebooks))
	for _, notebook := range result.Notebooks {
		output, err := l.buildNotebookSummary(ctx, notebook)
		if err != nil {
			return nil, errors.WithMessagef(err, "build notebook output failed, notebook_id=%s", notebook.Id)
		}
		notebooks = append(notebooks, output)
	}

	return &ListNotebooksResult{
		Notebooks: notebooks,
		HasMore:   result.HasMore,
	}, nil
}

type ListNotebookSourcesParams struct {
	NotebookId uuid.UUID
	Limit      int
	Offset     int
}

type ListNotebookSourcesResult struct {
	Sources []*model.FullSource
	HasMore bool
}

func (l *Logic) ListNotebookSources(
	ctx context.Context,
	params *ListNotebookSourcesParams,
) (*ListNotebookSourcesResult, error) {
	_, err := l.notebookBiz.GetNotebook(ctx, params.NotebookId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get notebook failed, notebook_id=%s", params.NotebookId)
	}

	fetchLimit := params.Limit + 1 // for has more check
	req := &bizsource.ListDecodedSourcesByNotebookQuery{
		NotebookId: params.NotebookId,
		Limit:      fetchLimit,
		Offset:     params.Offset,
	}
	sources, err := l.sourceBiz.ListDecodedSourcesByNotebook(
		ctx,
		req,
		bizsource.WithContentRefUrl(true),
	)
	if err != nil {
		return nil, errors.WithMessagef(err, "list notebook sources failed, notebook_id=%s", params.NotebookId)
	}

	hasMore := len(sources) > params.Limit
	if hasMore {
		sources = sources[:params.Limit]
	}

	// populate url
	fullSources := make([]*model.FullSource, 0, len(sources))
	for _, source := range sources {
		fullSource := &model.FullSource{
			DecodedSource: source,
		}
		fullSources = append(fullSources, fullSource)
	}

	err = l.sourceBiz.BatchPopulateFullSources(ctx, fullSources)
	if err != nil {
		slog.ErrorContext(ctx, "batch populate full sources failed",
			slog.Any("err", err),
			slog.String("notebook_id", params.NotebookId.String()),
		)
	}

	return &ListNotebookSourcesResult{
		Sources: fullSources,
		HasMore: hasMore,
	}, nil
}

func (l *Logic) buildNotebookSummary(
	ctx context.Context,
	notebook *model.Notebook,
) (*NotebookSummary, error) {
	sourceCount, err := l.sourceBiz.CountSourcesByNotebook(ctx, notebook.Id)
	if err != nil {
		return nil, errors.WithMessagef(err, "count sources failed, notebook_id=%s", notebook.Id)
	}

	return &NotebookSummary{
		Notebook:    notebook,
		SourceCount: sourceCount,
	}, nil
}

// func (l *Logic) UpdateNotebookName(
// 	ctx context.Context,
// 	id uuid.UUID,
// 	name string,
// ) error {
// 	_, err := l.notebookBiz.GetNotebook(ctx, id)
// 	if err != nil {
// 		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
// 			return errors.ErrParams.Msgf("notebook not found, notebook_id=%s", id)
// 		}
// 		return errors.WithMessage(err, "update notebook name failed")
// 	}

// 	err = l.notebookBiz.UpdateNotebookName(ctx, id, name)
// 	if err != nil {
// 		return errors.WithMessage(err, "update notebook name failed")
// 	}

// 	return nil
// }

func (l *Logic) UpdateNotebookDescription(
	ctx context.Context,
	id uuid.UUID,
	desc string,
) error {
	_, err := l.notebookBiz.GetNotebook(ctx, id)
	if err != nil {
		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
			return errors.ErrParams.Msgf("notebook not found, notebook_id=%s", id)
		}
		return errors.WithMessage(err, "update notebook desc failed")
	}

	err = l.notebookBiz.UpdateNotebookDescription(ctx,
		&biznotebook.UpdateNotebookDescriptionCommand{
			Id:          id,
			Description: desc,
		})
	if err != nil {
		return errors.WithMessage(err, "update notebook desc failed")
	}

	return nil
}

func (l *Logic) GetOrCreateNotebookChat(
	ctx context.Context,
	id uuid.UUID,
) (*chatmodel.Chat, error) {
	userId := pkgcontext.GetUserId(ctx)
	chat, err := l.chatBiz.CreateIfAbsent(ctx,
		&bizchat.CreateIfAbsentCommand{
			NotebookId: id,
			UserId:     userId,
		})
	if err != nil {
		return nil, errors.WithMessage(err, "create notebook chat failed")
	}

	return chat, nil
}

// TODO 需要进行领域事件的通知改造
func (l *Logic) DeleteNotebook(
	ctx context.Context,
	id uuid.UUID,
) error {
	_, err := l.notebookBiz.GetNotebook(ctx, id)
	if err != nil {
		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
			return nil
		}

		return errors.WithMessagef(err, "get notebook failed before deleting, notebook_id=%s", id)
	}

	err = l.notebookBiz.DeleteNotebook(ctx, id)
	if err != nil {
		return errors.WithMessagef(err, "delete notebook failed, notebook_id=%s", id)
	}

	err = l.sourceBiz.DeleteNotebookSources(ctx, id)
	if err != nil {
		slog.ErrorContext(ctx,
			"delete notebook sources failed",
			slog.Any("err", err),
			slog.String("notebook_id", id.String()),
		)
	}

	err = l.chatBiz.DeleteNotebookChats(ctx, id)
	if err != nil {
		slog.ErrorContext(ctx,
			"delete notebook chats failed",
			slog.Any("err", err),
			slog.String("notebook_id", id.String()),
		)
	}

	err = l.artifactBiz.DeleteNotebookTasks(ctx, id)
	if err != nil {
		slog.ErrorContext(ctx,
			"delete notebook tasks failed",
			slog.Any("err", err),
			slog.String("notebook_id", id.String()),
		)
	}

	return nil
}

func (l *Logic) CheckNotebookUserId(ctx context.Context, id uuid.UUID) error {
	userId := pkgcontext.GetUserId(ctx)
	notebookUserId, err := l.notebookBiz.GetNotebookUser(ctx, id)
	if err != nil {
		return errors.WithMessagef(err, "get notebook user failed, notebook_id=%s", id)
	}
	if notebookUserId != userId {
		return errors.ErrPermission.Msgf("notebook access denied, notebook_id=%s", id)
	}
	return nil
}
