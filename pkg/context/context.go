package context

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/pkg/requestid"
	"github.com/gonotelm-lab/gonotelm/pkg/ulid"
)

type contextKey string

const (
	ContextKeyReqId        = contextKey("_req_id")
	ContextKeyUserId       = contextKey("_user_id")
	ContextKeySceneType    = contextKey("_scene_type")
	ContextKeySceneGroupId = contextKey("_scene_group_id")
	ContextLang            = contextKey("_lang")
	ContextOperatorType    = contextKey("_operator_type")
)

func WithReqId(ctx context.Context, reqId requestid.ID) context.Context {
	return context.WithValue(ctx, ContextKeyReqId, reqId)
}

func GetReqId(ctx context.Context) requestid.ID {
	reqId, ok := ctx.Value(ContextKeyReqId).(requestid.ID)
	if !ok {
		return requestid.ID{}
	}

	return reqId
}

func WithUserId(ctx context.Context, userId ulid.ULID) context.Context {
	return context.WithValue(ctx, ContextKeyUserId, userId)
}

func GetUserId(ctx context.Context) ulid.ULID {
	userId, ok := ctx.Value(ContextKeyUserId).(ulid.ULID)
	if !ok {
		return ulid.EmptyULID()
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

func WithSceneGroupId(ctx context.Context, sceneGroupId SceneGroupId) context.Context {
	return context.WithValue(ctx, ContextKeySceneGroupId, sceneGroupId)
}

func GetSceneGroupId(ctx context.Context) SceneGroupId {
	sceneGroupId, ok := ctx.Value(ContextKeySceneGroupId).(SceneGroupId)
	if ok {
		return sceneGroupId
	}

	return ""
}

func WithScene(ctx context.Context, sceneType SceneType, id SceneGroupId) context.Context {
	return WithSceneGroupId(WithSceneType(ctx, sceneType), id)
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

func WithOperatorType(ctx context.Context, operatorType OperatorType) context.Context {
	return context.WithValue(ctx, ContextOperatorType, operatorType)
}

func GetOperatorType(ctx context.Context) OperatorType {
	operatorType, ok := ctx.Value(ContextOperatorType).(OperatorType)
	if !ok {
		return OperatorTypeUser
	}

	return operatorType
}

func WithAgentOperate(ctx context.Context) context.Context {
	return context.WithValue(ctx, ContextOperatorType, OperatorTypeAgent)
}

func GetAgentOperate(ctx context.Context) bool {
	return GetOperatorType(ctx) == OperatorTypeAgent
}
