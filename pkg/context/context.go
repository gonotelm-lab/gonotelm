package context

import "context"

type contextKey string

const (
	ContextKeyUserId  = contextKey("_user_id")
	ContextKeyBizType = contextKey("_biz_type")
)

func WithUserId(ctx context.Context, userId string) context.Context {
	return context.WithValue(ctx, ContextKeyUserId, userId)
}

func GetUserId(ctx context.Context) string {
	userId, ok := ctx.Value(ContextKeyUserId).(string)
	if !ok {
		return ""
	}

	return userId
}

func WithBizType(ctx context.Context, bizType SceneType) context.Context {
	return context.WithValue(ctx, ContextKeyBizType, bizType)
}

func GetBizType(ctx context.Context) SceneType {
	bizType, ok := ctx.Value(ContextKeyBizType).(SceneType)
	if !ok {
		return UnknownScene
	}

	return bizType
}
