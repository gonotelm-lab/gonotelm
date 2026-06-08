package context

import "context"

type contextKey string

const (
	ContextKeyUserId    = contextKey("_user_id")
	ContextKeySceneType = contextKey("_scene_type")
	ContextLang         = contextKey("_lang")
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

func WithSceneType(ctx context.Context, sceneType SceneType) context.Context {
	return context.WithValue(ctx, ContextKeySceneType, sceneType)
}

func GetSceneType(ctx context.Context) SceneType {
	sceneType, ok := ctx.Value(ContextKeySceneType).(SceneType)
	if !ok {
		return UnknownScene
	}

	return sceneType
}

func WithLang(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, ContextLang, lang)
}

func GetLang(ctx context.Context) string {
	lang, ok := ctx.Value(ContextLang).(string)
	if !ok {
		return ""
	}

	return lang
}
