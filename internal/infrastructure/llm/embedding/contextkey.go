package embedding

import (
	"context"
	"time"

	"github.com/cloudwego/eino/components/embedding"
)

type (
	modelNameCtxKey    struct{}
	onStartInputCtxKey struct{}
	providerCtxKey     struct{}
	startTimeCtxKey    struct{}
)

func withProvider(ctx context.Context, provider EmbeddingType) context.Context {
	return context.WithValue(ctx, providerCtxKey{}, provider)
}

func getProvider(ctx context.Context) EmbeddingType {
	provider, ok := ctx.Value(providerCtxKey{}).(EmbeddingType)
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

func withOnStartInput(ctx context.Context, input *embedding.CallbackInput) context.Context {
	return context.WithValue(ctx, onStartInputCtxKey{}, input)
}

func getOnStartInput(ctx context.Context) *embedding.CallbackInput {
	v, ok := ctx.Value(onStartInputCtxKey{}).(*embedding.CallbackInput)
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
