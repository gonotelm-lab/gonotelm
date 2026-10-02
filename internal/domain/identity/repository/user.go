package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
)

type UserRepository interface {
	Save(ctx context.Context, user *entity.User) error
	GetById(ctx context.Context, id valobj.Uid) (*entity.User, error)
	GetByProviderSub(ctx context.Context, provider entity.ProviderType, sub string) (*entity.User, error)
}

type UserSessionRepository interface {
	Save(ctx context.Context, session *entity.UserSession) error
	Get(ctx context.Context, id string) (*entity.UserSession, error)
	Delete(ctx context.Context, id string) error
	// DeleteByUserId 作废该用户的所有会话
	DeleteByUserId(ctx context.Context, userId valobj.Uid) error
}
