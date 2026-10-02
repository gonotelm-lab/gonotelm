package service

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityerrors "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UserService struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

type RegisterParams struct {
	Provider entity.ProviderType
	Subject  string
	Nickname string
}

func (s *UserService) GetOrRegister(ctx context.Context, params RegisterParams) (*entity.User, error) {
	user, err := s.userRepo.GetByProviderSub(ctx, params.Provider, params.Subject)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, identityerrors.ErrUserNotFound) {
		return nil, errors.WithMessagef(err, "get user failed")
	}

	user = entity.NewUser(params.Nickname, params.Provider, params.Subject)
	if err := s.userRepo.Save(ctx, user); err != nil {
		// 并发下可能已被其他请求注册，回查一次
		if existing, getErr := s.userRepo.GetByProviderSub(ctx, params.Provider, params.Subject); getErr == nil {
			return existing, nil
		}

		return nil, errors.WithMessage(err, "save user failed")
	}

	return user, nil
}
