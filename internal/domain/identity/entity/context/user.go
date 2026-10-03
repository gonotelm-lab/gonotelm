package context

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
)

type userCtxKey struct{}

func WithUser(ctx context.Context, user *entity.User) context.Context {
	return context.WithValue(ctx, userCtxKey{}, user)
}

func GetUser(ctx context.Context) *entity.User {
	v := ctx.Value(userCtxKey{})
	u, ok := v.(*entity.User)
	if ok {
		return u
	}

	return nil
}
