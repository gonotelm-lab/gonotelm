package service

import (
	"context"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/entity"
	favouriteerrors "github.com/gonotelm-lab/gonotelm/internal/domain/favourite/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeFavouriteRepo struct {
	repository.FavouriteRepository
	count int64
	err   error
}

func (f *fakeFavouriteRepo) CountByUserId(context.Context, valobj.Uid) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.count, nil
}

type fakeFavouriteNotebookRepo struct {
	repository.FavouriteNotebookRepository
	count int64
	err   error
}

func (f *fakeFavouriteNotebookRepo) CountByFavouriteId(context.Context, valobj.Id) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.count, nil
}

func TestCheckFavouriteLimitUnderLimit(t *testing.T) {
	s := New(&fakeFavouriteRepo{count: int64(entity.MaxFavouritesPerUser - 1)}, &fakeFavouriteNotebookRepo{})
	err := s.CheckFavouriteLimit(context.Background(), valobj.NewUid())
	require.NoError(t, err)
}

func TestCheckFavouriteLimitAtLimit(t *testing.T) {
	s := New(&fakeFavouriteRepo{count: int64(entity.MaxFavouritesPerUser)}, &fakeFavouriteNotebookRepo{})
	err := s.CheckFavouriteLimit(context.Background(), valobj.NewUid())
	require.Error(t, err)
	assert.True(t, errors.Is(err, favouriteerrors.ErrFavouriteLimitExceeded))
}

func TestCheckNotebookLimitAtLimit(t *testing.T) {
	s := New(&fakeFavouriteRepo{}, &fakeFavouriteNotebookRepo{count: int64(entity.MaxNotebooksPerFavourite)})
	err := s.CheckNotebookLimit(context.Background(), valobj.NewId())
	require.Error(t, err)
	assert.True(t, errors.Is(err, favouriteerrors.ErrFavouriteNotebookLimitExceeded))
}

func TestCheckFavouriteLimitCountFailed(t *testing.T) {
	s := New(&fakeFavouriteRepo{err: errors.ErrInner.Msg("boom")}, &fakeFavouriteNotebookRepo{})
	err := s.CheckFavouriteLimit(context.Background(), valobj.NewUid())
	require.Error(t, err)
	assert.False(t, errors.Is(err, favouriteerrors.ErrFavouriteLimitExceeded))
}
