package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// UnbindHandler 解绑三方身份：bind_id 必须属于当前账号，且 provider 与记录一致。
type UnbindHandler struct {
	userService  *identityservice.UserService
	identityRepo identrepo.UserIdentityRepository
}

func NewUnbindHandler(
	userService *identityservice.UserService,
	identityRepo identrepo.UserIdentityRepository,
) *UnbindHandler {
	return &UnbindHandler{userService: userService, identityRepo: identityRepo}
}

type UnbindCommand struct {
	BindId       valobj.Id
	ProviderType entity.ProviderType
	UserId       valobj.Uid
}

func (h *UnbindHandler) Handle(ctx context.Context, cmd *UnbindCommand) error {
	if err := h.userService.EnsureNotLastLoginMethod(ctx, cmd.UserId); err != nil {
		return err
	}

	identity, err := h.identityRepo.GetByUserIdAndId(ctx, cmd.UserId, cmd.BindId)
	if err != nil {
		if errors.Is(err, identerr.ErrIdentityNotFound) {
			return identerr.ErrIdentityNotFound
		}

		return errors.WithMessage(err, "get user identity failed")
	}

	if identity.Provider != cmd.ProviderType {
		return errors.ErrParams.Msgf("provider mismatch: binding is %s", identity.Provider)
	}

	if err := h.identityRepo.Delete(ctx, identity.Id); err != nil {
		return errors.WithMessage(err, "delete identity failed")
	}

	return nil
}
