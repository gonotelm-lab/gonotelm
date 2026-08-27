package chat

import (
	"context"
	"time"

	"github.com/cloudwego/eino/components/model"
)

type (
	modelNameCtxKey      struct{}
	semReleaseFuncCtxKey struct{}
	streamingCtxKey      struct{}
	thinkingCtxKey       struct{}
	jsonObjectCtxKey     struct{}
	onStartInputCtxKey   struct{}
	providerCtxKey       struct{}
	startTimeCtxKey      struct{}
)

func withStreaming(ctx context.Context, isStreaming bool) context.Context {
	return context.WithValue(ctx, streamingCtxKey{}, isStreaming)
}

func getStreaming(ctx context.Context) bool {
	streaming, ok := ctx.Value(streamingCtxKey{}).(bool)
	if !ok {
		return false
	}

	return streaming
}

func withThinking(ctx context.Context, enableThinking bool) context.Context {
	return context.WithValue(ctx, thinkingCtxKey{}, enableThinking)
}

func getThinking(ctx context.Context) (bool, bool) {
	thinking, ok := ctx.Value(thinkingCtxKey{}).(bool)
	return thinking, ok
}

func withJSONObject(ctx context.Context, jsonObject bool) context.Context {
	return context.WithValue(ctx, jsonObjectCtxKey{}, jsonObject)
}

func getJSONObject(ctx context.Context) bool {
	jsonObject, ok := ctx.Value(jsonObjectCtxKey{}).(bool)
	if !ok {
		return false
	}

	return jsonObject
}

func withProvider(ctx context.Context, provider Provider) context.Context {
	return context.WithValue(ctx, providerCtxKey{}, provider)
}

func getProvider(ctx context.Context) Provider {
	provider, ok := ctx.Value(providerCtxKey{}).(Provider)
	if !ok {
		return ""
	}

	return provider
}

func withModelName(ctx context.Context, modelName string) context.Context {
	return context.WithValue(ctx, modelNameCtxKey{}, modelName)
}

func getModelName(ctx context.Context) string {
	modelName, ok := ctx.Value(modelNameCtxKey{}).(string)
	if !ok {
		return ""
	}

	return modelName
}

func withSemReleaseFunc(ctx context.Context, release func()) context.Context {
	if release == nil {
		return ctx
	}

	return context.WithValue(ctx, semReleaseFuncCtxKey{}, release)
}

func getSemReleaseFunc(ctx context.Context) func() {
	release, ok := ctx.Value(semReleaseFuncCtxKey{}).(func())
	if !ok || release == nil {
		return nil
	}

	return release
}

func runSemRelease(ctx context.Context) {
	release := getSemReleaseFunc(ctx)
	if release == nil {
		return
	}

	release()
}

func withOnStartInput(ctx context.Context, input *model.CallbackInput) context.Context {
	return context.WithValue(ctx, onStartInputCtxKey{}, input)
}

func getOnStartInput(ctx context.Context) *model.CallbackInput {
	v, ok := ctx.Value(onStartInputCtxKey{}).(*model.CallbackInput)
	if ok {
		return v
	}
	return nil
}

func withStartTime(ctx context.Context, start time.Time) context.Context {
	return context.WithValue(ctx, startTimeCtxKey{}, start)
}

func getStartTime(ctx context.Context) time.Time {
	t, ok := ctx.Value(startTimeCtxKey{}).(time.Time)
	if ok {
		return t
	}

	return time.Time{}
}
