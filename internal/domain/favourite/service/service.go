package service

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/entity"
	favouriteerrors "github.com/gonotelm-lab/gonotelm/internal/domain/favourite/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type Service struct {
	favouriteRepo         repository.FavouriteRepository
	favouriteNotebookRepo repository.FavouriteNotebookRepository
}

func New(
	favouriteRepo repository.FavouriteRepository,
	favouriteNotebookRepo repository.FavouriteNotebookRepository,
) *Service {
	return &Service{
		favouriteRepo:         favouriteRepo,
		favouriteNotebookRepo: favouriteNotebookRepo,
	}
}

// CheckFavouriteLimit 检查用户收藏夹额度；已达上限返回 ErrFavouriteLimitExceeded。
func (s *Service) CheckFavouriteLimit(ctx context.Context, userId valobj.Uid) error {
	count, err := s.favouriteRepo.CountByUserId(ctx, userId)
	if err != nil {
		return errors.WithMessage(err, "count favourites failed")
	}
	if count >= int64(entity.MaxFavouritesPerUser) {
		return favouriteerrors.ErrFavouriteLimitExceeded
	}
	return nil
}

// CheckNotebookLimit 检查单夹笔记本额度；已达上限返回 ErrFavouriteNotebookLimitExceeded。
func (s *Service) CheckNotebookLimit(ctx context.Context, favouriteId valobj.Id) error {
	count, err := s.favouriteNotebookRepo.CountByFavouriteId(ctx, favouriteId)
	if err != nil {
		return errors.WithMessage(err, "count favourite notebooks failed")
	}
	if count >= int64(entity.MaxNotebooksPerFavourite) {
		return favouriteerrors.ErrFavouriteNotebookLimitExceeded
	}
	return nil
}
