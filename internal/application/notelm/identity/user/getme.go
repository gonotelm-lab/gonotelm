package user

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

type GetMeHandler struct {
	userService *identityservice.UserService
	objectStore adapter.ObjectPublicURLer
}

func NewGetMeHandler(
	userService *identityservice.UserService,
	objectStore adapter.ObjectPublicURLer,
) *GetMeHandler {
	return &GetMeHandler{userService: userService, objectStore: objectStore}
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
	agg, err := h.userService.GetAccount(ctx, pkgcontext.GetUserId(ctx))
	if err != nil {
		return nil, err
	}

	user := agg.User()

	return &MeResult{
		UserId:      user.Id.String(),
		Nickname:    user.Nickname,
		Email:       user.Email,
		AvatarUrl:   h.avatarURL(ctx, user.Avatar),
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		LoginSource: h.loginSource(ctx, agg),
		HasPassword: agg.HasPassword(),
	}, nil
}

// loginSource 优先取会话携带的登录方式，旧会话缺该字段时改用最早绑定的身份
func (h *GetMeHandler) loginSource(ctx context.Context, agg *entity.UserAggregate) string {
	if source := pkgcontext.GetLoginSource(ctx); source != "" {
		return source
	}

	identities := agg.Identities()
	if len(identities) == 0 {
		return ""
	}

	return identities[0].Provider.String()
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
