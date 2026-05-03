package logic

import (
	"context"

	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type NotebookLogic struct {
	notebookBiz *biznotebook.Biz
	sourceBiz   *bizsource.Biz
}

func NewNotebookLogic(
	notebookBiz *biznotebook.Biz,
	sourceBiz *bizsource.Biz,
) *NotebookLogic {
	return &NotebookLogic{
		notebookBiz: notebookBiz,
		sourceBiz:   sourceBiz,
	}
}

type CreateNotebookParams struct {
	Name string
	Desc string
}

func (l *NotebookLogic) CreateNotebook(
	ctx context.Context,
	params *CreateNotebookParams,
) (*model.Notebook, error) {
	notebook, err := l.notebookBiz.CreateNotebook(
		ctx, &biznotebook.CreateNotebookCommand{
			Name: params.Name,
			Desc: params.Desc,
		})
	if err != nil {
		return nil, errors.WithMessage(err, "create notebook failed")
	}

	return notebook, nil
}

type NotebookSummary struct {
	Notebook    *model.Notebook
	SourceCount int64
}

func (l *NotebookLogic) GetNotebook(
	ctx context.Context,
	id uuid.UUID,
) (*NotebookSummary, error) {
	notebook, err := l.notebookBiz.GetNotebook(ctx, id)
	if err != nil {
		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
			return nil, errors.ErrParams.Msgf("notebook not found, notebook_id=%s", id)
		}

		return nil, errors.WithMessage(err, "get notebook failed")
	}

	output, err := l.buildNotebookSummary(ctx, notebook)
	if err != nil {
		return nil, errors.WithMessage(err, "build notebook output failed")
	}

	return output, nil
}

type ListNotebooksParams struct {
	Limit  int
	Offset int
}

type ListNotebooksResult struct {
	Notebooks []*NotebookSummary
	HasMore   bool
}

func (l *NotebookLogic) ListNotebooks(
	ctx context.Context,
	params *ListNotebooksParams,
) (*ListNotebooksResult, error) {
	result, err := l.notebookBiz.ListNotebooks(
		ctx,
		&biznotebook.ListNotebooksQuery{
			Limit:  params.Limit,
			Offset: params.Offset,
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
	Sources []*model.SourceWithContent
	HasMore bool
}

func (l *NotebookLogic) ListNotebookSources(
	ctx context.Context,
	params *ListNotebookSourcesParams,
) (*ListNotebookSourcesResult, error) {
	_, err := l.notebookBiz.GetNotebook(ctx, params.NotebookId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get notebook failed, notebook_id=%s", params.NotebookId)
	}

	fetchLimit := params.Limit + 1
	sources, err := l.sourceBiz.ListSourcesByNotebook(
		ctx,
		params.NotebookId,
		fetchLimit,
		params.Offset)
	if err != nil {
		return nil, errors.WithMessagef(err, "list notebook sources failed, notebook_id=%s", params.NotebookId)
	}

	hasMore := len(sources) > params.Limit
	if hasMore {
		sources = sources[:params.Limit]
	}

	return &ListNotebookSourcesResult{
		Sources: sources,
		HasMore: hasMore,
	}, nil
}

func (l *NotebookLogic) buildNotebookSummary(
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

func (l *NotebookLogic) UpdateNotebookName(
	ctx context.Context,
	id uuid.UUID,
	name string,
) error {
	_, err := l.notebookBiz.GetNotebook(ctx, id)
	if err != nil {
		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
			return errors.ErrParams.Msgf("notebook not found, notebook_id=%s", id)
		}
		return errors.WithMessage(err, "update notebook name failed")
	}

	err = l.notebookBiz.UpdateNotebookName(ctx, id, name)
	if err != nil {
		return errors.WithMessage(err, "update notebook name failed")
	}

	return nil
}

func (l *NotebookLogic) UpdateNotebookDesc(
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

	err = l.notebookBiz.UpdateNotebookDesc(ctx, id, desc)
	if err != nil {
		return errors.WithMessage(err, "update notebook desc failed")
	}

	return nil
}
