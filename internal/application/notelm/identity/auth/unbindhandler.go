package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
)

// UnbindHandler 解绑三方身份：bind_id 必须属于当前账号，且 provider 与记录一致。
type UnbindHandler struct {
	userService *identityservice.UserService
}

func NewUnbindHandler(userService *identityservice.UserService) *UnbindHandler {
	return &UnbindHandler{userService: userService}
}

type UnbindCommand struct {
	BindId       valobj.Id
	ProviderType entity.ProviderType
	UserId       valobj.Uid
}

func (h *UnbindHandler) Handle(ctx context.Context, cmd *UnbindCommand) error {
	agg, err := h.userService.GetAccount(ctx, cmd.UserId)
	if err != nil {
		return err
	}

	return h.userService.UnbindIdentity(ctx, agg, cmd.BindId, cmd.ProviderType)
}
