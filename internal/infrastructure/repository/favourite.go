package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/entity"
	favouriteerrors "github.com/gonotelm-lab/gonotelm/internal/domain/favourite/errors"
	favouriterepo "github.com/gonotelm-lab/gonotelm/internal/domain/favourite/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type FavouriteRepositoryImpl struct {
	favouriteStore         database.FavouriteStore
	favouriteNotebookStore database.FavouriteNotebookStore
	transactor             database.Transactor
}

func NewFavouriteRepository(
	favouriteStore database.FavouriteStore,
	favouriteNotebookStore database.FavouriteNotebookStore,
	transactor database.Transactor,
) favouriterepo.FavouriteRepository {
	return &FavouriteRepositoryImpl{
		favouriteStore:         favouriteStore,
		favouriteNotebookStore: favouriteNotebookStore,
		transactor:             transactor,
	}
}

var _ favouriterepo.FavouriteRepository = &FavouriteRepositoryImpl{}

func (r *FavouriteRepositoryImpl) Save(ctx context.Context, f *entity.Favourite) error {
	if f.IsDeleted() {
		return r.transactor.Transaction(ctx, func(txCtx context.Context) error {
			if err := r.favouriteNotebookStore.DeleteByFavouriteId(txCtx, f.Id); err != nil {
				return err
			}
			return r.favouriteStore.DeleteById(txCtx, f.Id)
		})
	}
	return r.favouriteStore.Upsert(ctx, mapper.FavouriteToSchema(f))
}

func (r *FavouriteRepositoryImpl) Find(ctx context.Context, id valobj.Id) (*entity.Favourite, error) {
	row, err := r.favouriteStore.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, favouriteerrors.ErrFavouriteNotFound
		}
		return nil, err
	}
	return mapper.FavouriteFromSchema(row), nil
}

func (r *FavouriteRepositoryImpl) ListByUserId(
	ctx context.Context,
	userId valobj.Uid,
	spec *favouriterepo.ListSpec,
) ([]*entity.Favourite, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	rows, err := r.favouriteStore.ListByUserId(ctx, userId, spec.Limit, spec.Offset)
	if err != nil {
		return nil, err
	}
	return mapper.FavouritesFromSchema(rows), nil
}

func (r *FavouriteRepositoryImpl) CountByUserId(ctx context.Context, userId valobj.Uid) (int64, error) {
	return r.favouriteStore.CountByUserId(ctx, userId)
}

func (r *FavouriteRepositoryImpl) DeleteByUserId(ctx context.Context, userId valobj.Uid) error {
	return r.transactor.Transaction(ctx, func(txCtx context.Context) error {
		if err := r.favouriteNotebookStore.DeleteByUserId(txCtx, userId); err != nil {
			return err
		}
		return r.favouriteStore.DeleteByUserId(txCtx, userId)
	})
}

type FavouriteNotebookRepositoryImpl struct {
	favouriteNotebookStore database.FavouriteNotebookStore
}

func NewFavouriteNotebookRepository(
	favouriteNotebookStore database.FavouriteNotebookStore,
) favouriterepo.FavouriteNotebookRepository {
	return &FavouriteNotebookRepositoryImpl{favouriteNotebookStore: favouriteNotebookStore}
}

var _ favouriterepo.FavouriteNotebookRepository = &FavouriteNotebookRepositoryImpl{}

func (r *FavouriteNotebookRepositoryImpl) Save(
	ctx context.Context,
	row *entity.FavouriteNotebook,
) error {
	return r.favouriteNotebookStore.InsertIgnore(ctx, mapper.FavouriteNotebookToSchema(row))
}

func (r *FavouriteNotebookRepositoryImpl) Find(
	ctx context.Context,
	favouriteId, notebookId valobj.Id,
) (*entity.FavouriteNotebook, error) {
	row, err := r.favouriteNotebookStore.Get(ctx, favouriteId, notebookId)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, favouriteerrors.ErrFavouriteNotebookNotFound
		}
		return nil, err
	}
	return mapper.FavouriteNotebookFromSchema(row), nil
}

func (r *FavouriteNotebookRepositoryImpl) ListByFavouriteId(
	ctx context.Context,
	favouriteId valobj.Id,
	spec *favouriterepo.ListSpec,
) ([]*entity.FavouriteNotebook, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	rows, err := r.favouriteNotebookStore.ListByFavouriteId(ctx, favouriteId, spec.Limit, spec.Offset)
	if err != nil {
		return nil, err
	}
	return mapper.FavouriteNotebooksFromSchema(rows), nil
}

func (r *FavouriteNotebookRepositoryImpl) ListFavouritesByUserAndNotebook(
	ctx context.Context,
	userId valobj.Uid,
	notebookId valobj.Id,
) ([]*entity.Favourite, error) {
	rows, err := r.favouriteNotebookStore.ListFavouritesByUserAndNotebook(ctx, userId, notebookId)
	if err != nil {
		return nil, err
	}
	return mapper.FavouritesFromSchema(rows), nil
}

func (r *FavouriteNotebookRepositoryImpl) CountByFavouriteId(
	ctx context.Context,
	favouriteId valobj.Id,
) (int64, error) {
	return r.favouriteNotebookStore.CountByFavouriteId(ctx, favouriteId)
}

func (r *FavouriteNotebookRepositoryImpl) Delete(
	ctx context.Context,
	favouriteId valobj.Id,
	notebookIds []valobj.Id,
) error {
	ids := make([]database.Id, 0, len(notebookIds))
	for _, id := range notebookIds {
		ids = append(ids, id)
	}
	return r.favouriteNotebookStore.Delete(ctx, favouriteId, ids)
}

func (r *FavouriteNotebookRepositoryImpl) DeleteByFavouriteId(
	ctx context.Context,
	favouriteId valobj.Id,
) error {
	return r.favouriteNotebookStore.DeleteByFavouriteId(ctx, favouriteId)
}

func (r *FavouriteNotebookRepositoryImpl) DeleteByUserId(
	ctx context.Context,
	userId valobj.Uid,
) error {
	return r.favouriteNotebookStore.DeleteByUserId(ctx, userId)
}
