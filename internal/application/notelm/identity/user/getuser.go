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

type UserView struct {
	Id              string
	Nickname        string
	EmailVerifiedAt valobj.Time
	Status          entity.UserStatus
	AvatarURL       string
	CreatedAt       valobj.Time
	UpdatedAt       valobj.Time
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

	user, err := h.userService.GetUser(ctx, userId)
	if err != nil {
		return nil, err
	}

	return toUserView(ctx, h.storeKeyResolver, user), nil
}

func toUserView(ctx context.Context, resolver *storekey.Resolver, user *entity.User) *UserView {
	return &UserView{
		Id:              user.Id.String(),
		Nickname:        user.Nickname,
		EmailVerifiedAt: user.EmailVerifiedAt,
		Status:          user.Status,
		AvatarURL:       resolver.ResolvePublic(ctx, user.Avatar),
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
	}
}
