package auth

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// BindHandler 发起三方绑定：state 记录里带上发起会话，回调必须是同一个会话。
type BindHandler struct {
	registry     identrepo.ProviderRegistry
	stateRepo    identrepo.OAuthStateRepository
	userRepo     identrepo.UserRepository
	identityRepo identrepo.UserIdentityRepository
}

func NewBindHandler(
	registry identrepo.ProviderRegistry,
	stateRepo identrepo.OAuthStateRepository,
	userRepo identrepo.UserRepository,
	identityRepo identrepo.UserIdentityRepository,
) *BindHandler {
	return &BindHandler{
		registry:     registry,
		stateRepo:    stateRepo,
		userRepo:     userRepo,
		identityRepo: identityRepo,
	}
}

type BindCommand struct {
	ProviderType entity.ProviderType
	UserId       valobj.Uid
	SessionId    string
	ReturnTo     string
}

type BindResult struct {
	LoginInfo *entity.ProviderLoginInfo
	// AlreadyBound 该 provider 已经绑在当前账号上，不必再走一遍 OAuth
	AlreadyBound bool
}

func (h *BindHandler) Handle(ctx context.Context, cmd *BindCommand) (*BindResult, error) {
	user, err := h.userRepo.GetById(ctx, cmd.UserId)
	if err != nil {
		return nil, err
	}
	if !user.IsEmailVerified() {
		return nil, identerr.ErrEmailNotVerified
	}

	identities, err := h.identityRepo.ListByUserId(ctx, cmd.UserId)
	if err != nil {
		return nil, errors.WithMessage(err, "list user identities failed")
	}
	for _, identity := range identities {
		if identity.Provider == cmd.ProviderType {
			return &BindResult{AlreadyBound: true}, nil
		}
	}

	provider, err := h.registry.GetProvider(ctx, cmd.ProviderType)
	if err != nil {
		return nil, errors.WithMessage(err, "get login provider failed")
	}

	loginInfo, err := provider.LoginInfo(ctx, &entity.LoginInfoRequest{
		ReturnTo:  cmd.ReturnTo,
		Device:    entity.DeviceTypeWeb,
		Mode:      entity.ProviderLoginModeBind, // 绑定三方账户模式
		SessionId: cmd.SessionId,
	})
	if err != nil {
		return nil, errors.WithMessage(err, "get login info failed")
	}

	if err := h.stateRepo.Save(ctx, loginInfo.ToOAuthState()); err != nil {
		return nil, errors.WithMessage(err, "save login state failed")
	}

	return &BindResult{LoginInfo: loginInfo}, nil
}
