package context

import "context"

type contextKey string

const (
	ContextKeyUserId = contextKey("user_id")
)

func WithUserId(ctx context.Context, userId string) context.Context {
	return context.WithValue(ctx, ContextKeyUserId, userId)
}

func GetUserId(ctx context.Context) string {
	return ctx.Value(ContextKeyUserId).(string)
}
