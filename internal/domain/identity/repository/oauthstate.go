package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
)

// OAuthStateRepository 存取 OAuth 握手期间的一次性状态（state 为键，短 TTL）
// 登录与绑定共用，绑定模式下 state 里多了 mode 与发起会话
type OAuthStateRepository interface {
	Save(ctx context.Context, state *entity.OAuthState) error
	Get(ctx context.Context, state string) (*entity.OAuthState, error)
	Delete(ctx context.Context, state string) error
}
