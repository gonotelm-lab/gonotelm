package text2image

import (
	"context"
	"time"

	pkgt2i "github.com/gonotelm-lab/multimodal/image"
)

type (
	modelNameCtxKey    struct{}
	onStartInputCtxKey struct{}
	providerCtxKey     struct{}
	startTimeCtxKey    struct{}
)

func withProvider(ctx context.Context, provider Text2ImageProvider) context.Context {
	return context.WithValue(ctx, providerCtxKey{}, provider)
}

func getProvider(ctx context.Context) Text2ImageProvider {
	provider, ok := ctx.Value(providerCtxKey{}).(Text2ImageProvider)
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

func withOnStartInput(ctx context.Context, input *pkgt2i.CallbackInput) context.Context {
	return context.WithValue(ctx, onStartInputCtxKey{}, input)
}

func getOnStartInput(ctx context.Context) *pkgt2i.CallbackInput {
	v, ok := ctx.Value(onStartInputCtxKey{}).(*pkgt2i.CallbackInput)
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
