package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgslices "github.com/gonotelm-lab/gonotelm/pkg/slices"
)

type NotebookRepositoryImpl struct {
	notebookStore database.NotebookStore
	memberStore   database.NotebookMemberStore
	sourceStore   database.SourceStore
}

func NewNotebookRepository(
	notebookStore database.NotebookStore,
	memberStore database.NotebookMemberStore,
	sourceStore database.SourceStore,
) notebookrepo.Repository {
	return &NotebookRepositoryImpl{
		notebookStore: notebookStore,
		memberStore:   memberStore,
		sourceStore:   sourceStore,
	}
}

var _ notebookrepo.Repository = &NotebookRepositoryImpl{}

func (s *NotebookRepositoryImpl) Save(ctx context.Context, notebook *notebookentity.Notebook) error {
	if notebook.IsDeleted() {
		return s.notebookStore.DeleteById(ctx, notebook.Id)
	}

	sch := mapper.NotebookToSchema(notebook)
	return s.notebookStore.Upsert(ctx, sch)
}

func (s *NotebookRepositoryImpl) Exists(ctx context.Context, id valobj.Id) (bool, error) {
	return s.notebookStore.Exists(ctx, id)
}

func (s *NotebookRepositoryImpl) FindById(ctx context.Context, id valobj.Id) (*notebookentity.Notebook, error) {
	notebook, err := s.notebookStore.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, notebookerrors.ErrNotebookNotFound
		}

		return nil, err
	}

	sourceCount, err := s.sourceStore.CountByNotebookId(ctx, id)
	if err != nil {
		return nil, err
	}

	result := mapper.NotebookFromSchema(notebook)
	result.SourceCount = sourceCount

	return result, nil
}

func (s *NotebookRepositoryImpl) ListByIds(
	ctx context.Context,
	ids []valobj.Id,
) ([]*notebookentity.Notebook, error) {
	if len(ids) == 0 {
		return []*notebookentity.Notebook{}, nil
	}

	rows, err := s.notebookStore.ListByIds(ctx, ids)
	if err != nil {
		return nil, err
	}

	return s.withSourceCounts(ctx, rows)
}

func (s *NotebookRepositoryImpl) ListByOwner(
	ctx context.Context,
	ownerId valobj.Uid,
	spec *notebookrepo.ListSpec,
	excludeIds []valobj.Id,
) ([]*notebookentity.Notebook, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}

	rows, err := s.notebookStore.ListByOwnerId(
		ctx,
		ownerId,
		spec.Limit,
		spec.Offset,
		notebookListOrder(spec.Order),
		excludeIds,
	)
	if err != nil {
		return nil, err
	}

	return s.withSourceCounts(ctx, rows)
}

func (s *NotebookRepositoryImpl) ListShared(
	ctx context.Context,
	userId valobj.Uid,
	spec *notebookrepo.ListSpec,
	excludeIds []valobj.Id,
) ([]*notebookentity.Notebook, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}

	members, err := s.memberStore.ListByUserId(ctx, userId)
	if err != nil {
		return nil, err
	}

	exclude := make(map[valobj.Id]struct{}, len(excludeIds))
	for _, id := range excludeIds {
		exclude[id] = struct{}{}
	}

	notebookIds := make([]valobj.Id, 0, len(members))
	for _, member := range members {
		if _, skip := exclude[member.NotebookId]; skip {
			continue
		}
		notebookIds = append(notebookIds, member.NotebookId)
	}
	notebookIds = pkgslices.Unique(notebookIds)
	if len(notebookIds) == 0 {
		return []*notebookentity.Notebook{}, nil
	}

	// 只取 member 关联到的 notebook
	rows, err := s.notebookStore.ListByIdsExcludingOwner(
		ctx,
		notebookIds,
		userId,
		spec.Limit,
		spec.Offset,
		notebookListOrder(spec.Order),
	)
	if err != nil {
		return nil, err
	}

	return s.withSourceCounts(ctx, rows)
}

// notebookListOrder 把领域排序枚举翻译成 store 的排序参数。
func notebookListOrder(order notebookrepo.ListOrder) database.NotebookListOrder {
	if order == notebookrepo.ListOrderCreateTime {
		return database.NotebookListOrderCreateTime
	}

	return database.NotebookListOrderLastActive
}

func (s *NotebookRepositoryImpl) withSourceCounts(
	ctx context.Context,
	rows []*schema.Notebook,
) ([]*notebookentity.Notebook, error) {
	if len(rows) == 0 {
		return []*notebookentity.Notebook{}, nil
	}

	notebookIds := make([]valobj.Id, 0, len(rows))
	for _, row := range rows {
		notebookIds = append(notebookIds, row.Id)
	}

	counts, err := s.sourceStore.BatchCountByNotebookIds(ctx, notebookIds)
	if err != nil {
		return nil, err
	}

	notebooks := make([]*notebookentity.Notebook, 0, len(rows))
	for _, row := range rows {
		notebook := mapper.NotebookFromSchema(row)
		notebook.SourceCount = counts[row.Id]
		notebooks = append(notebooks, notebook)
	}

	return notebooks, nil
}
