package user

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/storekey"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type GetUserQuery struct {
	UserId string
}

type IdentityView struct {
	Id                    string
	Provider              entity.ProviderType
	Subject               string
	ProviderEmail         string
	ProviderEmailVerified bool
	BoundAt               valobj.Time
	LastLoginAt           valobj.Time
}

type UserView struct {
	Id              string
	Nickname        string
	EmailVerifiedAt valobj.Time
	Status          entity.UserStatus
	AvatarURL       string
	CreatedAt       valobj.Time
	UpdatedAt       valobj.Time
	HasPassword     bool
	Identities      []*IdentityView
}

type GetUserHandler struct {
	userService      *identityservice.UserService
	storeKeyResolver *storekey.Resolver
}

func NewGetUserHandler(
	userService *identityservice.UserService,
	objectStore adapter.ObjectPublicURLer,
) *GetUserHandler {
	return &GetUserHandler{
		userService:      userService,
		storeKeyResolver: storekey.New(objectStore),
	}
}

func (h *GetUserHandler) Handle(ctx context.Context, query *GetUserQuery) (*UserView, error) {
	userId, err := valobj.NewUidFromString(query.UserId)
	if err != nil {
		return nil, pkgerrors.ErrParams.Msg("invalid user id")
	}

	// GetAccount 带出密码标记与已绑定的三方身份
	agg, err := h.userService.GetAccount(ctx, userId)
	if err != nil {
		return nil, err
	}

	return toUserView(ctx, h.storeKeyResolver, agg), nil
}

func toUserView(ctx context.Context, resolver *storekey.Resolver, agg *entity.UserAggregate) *UserView {
	user := agg.User()
	identities := agg.Identities()
	views := make([]*IdentityView, 0, len(identities))
	for _, identity := range identities {
		views = append(views, &IdentityView{
			Id:                    identity.Id.String(),
			Provider:              identity.Provider,
			Subject:               identity.Subject,
			ProviderEmail:         identity.ProviderEmail,
			ProviderEmailVerified: identity.ProviderEmailVerified,
			BoundAt:               identity.CreatedAt,
			LastLoginAt:           identity.LastLoginAt,
		})
	}

	return &UserView{
		Id:              user.Id.String(),
		Nickname:        user.Nickname,
		EmailVerifiedAt: user.EmailVerifiedAt,
		Status:          user.Status,
		AvatarURL:       resolver.ResolvePublic(ctx, user.Avatar),
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
		HasPassword:     agg.HasPassword(),
		Identities:      views,
	}
}
