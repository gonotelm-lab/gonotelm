package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

// UnbindIdentityHandler 解绑三方身份：bind_id 必须属于当前账号，provider 与记录一致
type UnbindIdentityHandler struct {
	userService *identityservice.UserService
}

func NewUnbindIdentityHandler(userService *identityservice.UserService) *UnbindIdentityHandler {
	return &UnbindIdentityHandler{userService: userService}
}

type UnbindIdentityCommand struct {
	BindId       valobj.Id
	ProviderType entity.ProviderType
}

func (h *UnbindIdentityHandler) Handle(ctx context.Context, cmd *UnbindIdentityCommand) error {
	agg, err := h.userService.GetAccount(ctx, pkgcontext.GetUserId(ctx))
	if err != nil {
		return err
	}

	return h.userService.UnbindIdentity(ctx, agg, cmd.BindId, cmd.ProviderType)
}
