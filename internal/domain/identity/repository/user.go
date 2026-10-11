package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
)

type UserRepository interface {
	// 建号
	Create(ctx context.Context, user *entity.User) error
	// CreateWithPassword 建号并同时写入口令
	CreateWithPassword(ctx context.Context, user *entity.User, passwordHash string) error
	// CreateWithFirstIdentity 建号与挂第一个身份必须原子
	CreateWithFirstIdentity(ctx context.Context, user *entity.User, identity *entity.UserIdentity) error
	GetById(ctx context.Context, id valobj.Uid) (*entity.User, error)
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	GetPasswordHash(ctx context.Context, id valobj.Uid) (string, error)
	UpdateProfile(ctx context.Context, user *entity.User) error
	UpdateAvatar(ctx context.Context, user *entity.User) error
	UpdatePasswordHash(ctx context.Context, id valobj.Uid, hash string) error
}

type UserIdentityRepository interface {
	Create(ctx context.Context, identity *entity.UserIdentity) error
	GetByProviderAndSubject(ctx context.Context, provider entity.ProviderType, subject string) (*entity.UserIdentity, error)
	// GetByUserIdAndId 取该账号下的某条身份；不属于该账号时返回 ErrIdentityNotFound
	GetByUserIdAndId(ctx context.Context, userId valobj.Uid, id valobj.Id) (*entity.UserIdentity, error)
	ListByUserId(ctx context.Context, userId valobj.Uid) ([]*entity.UserIdentity, error)
	TouchLogin(ctx context.Context, identity *entity.UserIdentity) error
	// Delete 解绑单条身份；调用方先用 GetByUserIdAndId 确认归属
	Delete(ctx context.Context, id valobj.Id) error
	DeleteByUserId(ctx context.Context, userId valobj.Uid) error
}

type UserSessionRepository interface {
	Save(ctx context.Context, session *entity.UserSession) error
	Get(ctx context.Context, id string) (*entity.UserSession, error)
	Delete(ctx context.Context, id string) error
	// DeleteByUserId 作废该用户的所有会话
	DeleteByUserId(ctx context.Context, userId valobj.Uid) error
}
