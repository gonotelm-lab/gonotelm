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

// BindIdentityHandler 发起三方绑定：state 里记录发起会话，回调必须来自同一个会话
type BindIdentityHandler struct {
	registry       identrepo.ProviderRegistry
	oauthStateRepo identrepo.OAuthStateRepository
	userService    *identityservice.UserService
}

func NewBindIdentityHandler(
	registry identrepo.ProviderRegistry,
	oauthStateRepo identrepo.OAuthStateRepository,
	userService *identityservice.UserService,
) *BindIdentityHandler {
	return &BindIdentityHandler{
		registry:       registry,
		oauthStateRepo: oauthStateRepo,
		userService:    userService,
	}
}

type BindIdentityCommand struct {
	ProviderType entity.ProviderType
	UserId       valobj.Uid
	SessionId    string
	ReturnTo     string
}

type BindIdentityResult struct {
	LoginInfo *entity.ProviderOAuthInfo
	// AlreadyBound 该 provider 已经绑在当前账号上，不必再走一遍 OAuth
	AlreadyBound bool
}

func (h *BindIdentityHandler) Handle(ctx context.Context, cmd *BindIdentityCommand) (*BindIdentityResult, error) {
	agg, err := h.userService.GetAccount(ctx, cmd.UserId)
	if err != nil {
		return nil, err
	}
	if !agg.IsEmailVerified() {
		return nil, identerr.ErrEmailNotVerified
	}
	if agg.HasIdentity(cmd.ProviderType) {
		return &BindIdentityResult{AlreadyBound: true}, nil
	}

	provider, err := h.registry.GetProvider(ctx, cmd.ProviderType)
	if err != nil {
		return nil, errors.WithMessage(err, "get login provider failed")
	}

	oauthInfo, err := provider.OAuthInfo(ctx, &entity.OAuthInfoRequest{
		ReturnTo:  cmd.ReturnTo,
		Device:    entity.DeviceTypeWeb,
		Mode:      entity.ProviderLoginModeBind, // 绑定三方账号模式
		SessionId: cmd.SessionId,
	})
	if err != nil {
		return nil, errors.WithMessage(err, "get login info failed")
	}

	if err := h.oauthStateRepo.Save(ctx, oauthInfo.ToOAuthState()); err != nil {
		return nil, errors.WithMessage(err, "save login state failed")
	}

	return &BindIdentityResult{LoginInfo: oauthInfo}, nil
}
