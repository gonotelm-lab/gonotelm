package user

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UpdateProfileCommand struct {
	Nickname *string
}

type UpdateProfileHandler struct {
	userRepo repository.UserRepository
}

func NewUpdateProfileHandler(userRepo repository.UserRepository) *UpdateProfileHandler {
	return &UpdateProfileHandler{userRepo: userRepo}
}

// Handle 返回是否真实发生变更,无变更时跳过写入。
func (h *UpdateProfileHandler) Handle(ctx context.Context, cmd *UpdateProfileCommand) (bool, error) {
	user, err := h.userRepo.GetById(ctx, pkgcontext.GetUserId(ctx))
	if err != nil {
		return false, err
	}

	changed := false
	if cmd.Nickname != nil {
		if err := user.SetNickname(*cmd.Nickname); err != nil {
			return false, err
		}
		changed = true
	}

	if !changed {
		return false, nil
	}

	if err := h.userRepo.Save(ctx, user); err != nil {
		return false, errors.WithMessage(err, "update user profile failed")
	}

	return true, nil
}
