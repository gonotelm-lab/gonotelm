package service

import (
	"context"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	pinentity "github.com/gonotelm-lab/gonotelm/internal/domain/pin/entity"
	pinerrors "github.com/gonotelm-lab/gonotelm/internal/domain/pin/errors"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePinRepo struct {
	pinrepo.Repository
	count int64
	err   error
}

func (f *fakePinRepo) CountByUserId(context.Context, valobj.Uid) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.count, nil
}

func TestCheckPinnableUnderLimit(t *testing.T) {
	s := New(&fakePinRepo{count: int64(pinentity.MaxPinsAllowed - 1)})
	err := s.CheckPinnable(context.Background(), valobj.NewUid())
	require.NoError(t, err)
}

func TestCheckPinnableAtLimit(t *testing.T) {
	s := New(&fakePinRepo{count: int64(pinentity.MaxPinsAllowed)})
	err := s.CheckPinnable(context.Background(), valobj.NewUid())
	require.Error(t, err)
	assert.True(t, errors.Is(err, pinerrors.ErrPinLimitExceeded))
}

func TestCheckPinnableCountFailed(t *testing.T) {
	s := New(&fakePinRepo{err: errors.ErrInner.Msg("boom")})
	err := s.CheckPinnable(context.Background(), valobj.NewUid())
	require.Error(t, err)
	assert.False(t, errors.Is(err, pinerrors.ErrPinLimitExceeded))
}
