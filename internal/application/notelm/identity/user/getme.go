package user

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

type GetMeHandler struct {
	userRepo repository.UserRepository
}

func NewGetMeHandler(userRepo repository.UserRepository) *GetMeHandler {
	return &GetMeHandler{userRepo: userRepo}
}

type MeResult struct {
	UserId   string
	Nickname string
}

func (h *GetMeHandler) Handle(ctx context.Context) (*MeResult, error) {
	user, err := h.userRepo.GetById(ctx, pkgcontext.GetUserId(ctx))
	if err != nil {
		return nil, err
	}

	return &MeResult{
		UserId:   user.Id.String(),
		Nickname: user.Nickname,
	}, nil
}
