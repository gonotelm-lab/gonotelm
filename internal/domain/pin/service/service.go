package service

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	pinentity "github.com/gonotelm-lab/gonotelm/internal/domain/pin/entity"
	pinerrors "github.com/gonotelm-lab/gonotelm/internal/domain/pin/errors"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type Service struct {
	pinRepo pinrepo.Repository
}

func New(pinRepo pinrepo.Repository) *Service {
	return &Service{pinRepo: pinRepo}
}

// CheckPinnable 检查用户是否还能新增 pin；已达上限返回 ErrPinLimitExceeded。
func (s *Service) CheckPinnable(ctx context.Context, userId valobj.Uid) error {
	count, err := s.pinRepo.CountByUserId(ctx, userId)
	if err != nil {
		return errors.WithMessage(err, "count pins failed")
	}
	if count >= int64(pinentity.MaxPinsAllowed) {
		return pinerrors.ErrPinLimitExceeded
	}
	return nil
}
