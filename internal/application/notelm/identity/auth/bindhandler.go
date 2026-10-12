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

// BindHandler 发起三方绑定：state 记录里带上发起会话，回调必须是同一个会话。
type BindHandler struct {
	registry       identrepo.ProviderRegistry
	oauthStateRepo identrepo.OAuthStateRepository
	userService    *identityservice.UserService
}

func NewBindHandler(
	registry identrepo.ProviderRegistry,
	oauthStateRepo identrepo.OAuthStateRepository,
	userService *identityservice.UserService,
) *BindHandler {
	return &BindHandler{
		registry:       registry,
		oauthStateRepo: oauthStateRepo,
		userService:    userService,
	}
}

type BindCommand struct {
	ProviderType entity.ProviderType
	UserId       valobj.Uid
	SessionId    string
	ReturnTo     string
}

type BindResult struct {
	LoginInfo *entity.ProviderOAuthInfo
	// AlreadyBound 该 provider 已经绑在当前账号上，不必再走一遍 OAuth
	AlreadyBound bool
}

func (h *BindHandler) Handle(ctx context.Context, cmd *BindCommand) (*BindResult, error) {
	agg, err := h.userService.GetAccount(ctx, cmd.UserId)
	if err != nil {
		return nil, err
	}
	if !agg.IsEmailVerified() {
		return nil, identerr.ErrEmailNotVerified
	}
	if agg.HasIdentity(cmd.ProviderType) {
		return &BindResult{AlreadyBound: true}, nil
	}

	provider, err := h.registry.GetProvider(ctx, cmd.ProviderType)
	if err != nil {
		return nil, errors.WithMessage(err, "get login provider failed")
	}

	oauthInfo, err := provider.OAuthInfo(ctx, &entity.OAuthInfoRequest{
		ReturnTo:  cmd.ReturnTo,
		Device:    entity.DeviceTypeWeb,
		Mode:      entity.ProviderLoginModeBind, // 绑定三方账户模式
		SessionId: cmd.SessionId,
	})
	if err != nil {
		return nil, errors.WithMessage(err, "get login info failed")
	}

	if err := h.oauthStateRepo.Save(ctx, oauthInfo.ToOAuthState()); err != nil {
		return nil, errors.WithMessage(err, "save login state failed")
	}

	return &BindResult{LoginInfo: oauthInfo}, nil
}
