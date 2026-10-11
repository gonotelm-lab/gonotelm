package user

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

type GetMeHandler struct {
	userRepo     repository.UserRepository
	identityRepo repository.UserIdentityRepository
	objectStore  adapter.ObjectPublicURLer
}

func NewGetMeHandler(
	userRepo repository.UserRepository,
	identityRepo repository.UserIdentityRepository,
	objectStore adapter.ObjectPublicURLer,
) *GetMeHandler {
	return &GetMeHandler{userRepo: userRepo, identityRepo: identityRepo, objectStore: objectStore}
}

type MeResult struct {
	UserId      string
	Nickname    string
	Email       string
	AvatarUrl   string
	CreatedAt   valobj.Time
	UpdatedAt   valobj.Time
	LoginSource string
	HasPassword bool
}

func (h *GetMeHandler) Handle(ctx context.Context) (*MeResult, error) {
	user, err := h.userRepo.GetById(ctx, pkgcontext.GetUserId(ctx))
	if err != nil {
		return nil, err
	}

	return &MeResult{
		UserId:      user.Id.String(),
		Nickname:    user.Nickname,
		Email:       user.Email,
		AvatarUrl:   h.avatarURL(ctx, user.Avatar),
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		LoginSource: h.loginSource(ctx, user.Id),
		HasPassword: h.hasPassword(ctx, user.Id),
	}, nil
}

// loginSource 优先取会话携带的登录方式；旧会话缺该字段时退回最早绑定的身份。
func (h *GetMeHandler) loginSource(ctx context.Context, userId valobj.Uid) string {
	if source := pkgcontext.GetLoginSource(ctx); source != "" {
		return source
	}

	identities, err := h.identityRepo.ListByUserId(ctx, userId)
	if err != nil {
		slog.WarnContext(ctx, "list user identities failed", slog.Any("err", err))
		return ""
	}
	if len(identities) == 0 {
		return ""
	}

	return identities[0].Provider.String()
}

func (h *GetMeHandler) hasPassword(ctx context.Context, userId valobj.Uid) bool {
	hash, err := h.userRepo.GetPasswordHash(ctx, userId)
	if err != nil {
		slog.WarnContext(ctx, "read password hash failed", slog.Any("err", err))
		return false
	}

	return hash != ""
}

func (h *GetMeHandler) avatarURL(ctx context.Context, avatar valobj.StoreKey) string {
	if !avatar.Valid() {
		return ""
	}

	url, err := h.objectStore.PublicURL(ctx, avatar)
	if err != nil {
		slog.WarnContext(ctx, "resolve avatar public url failed",
			slog.String("avatar", avatar.String()),
			slog.Any("err", err),
		)
		return ""
	}

	return url
}
