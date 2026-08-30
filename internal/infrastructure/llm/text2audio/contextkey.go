package text2audio

import (
	"context"
	"time"

	audios "github.com/gonotelm-lab/multimodal/audio"
)

type (
	modelNameCtxKey    struct{}
	onStartInputCtxKey struct{}
	providerCtxKey     struct{}
	startTimeCtxKey    struct{}
)

func withProvider(ctx context.Context, provider Text2AudioProvider) context.Context {
	return context.WithValue(ctx, providerCtxKey{}, provider)
}

func getProvider(ctx context.Context) Text2AudioProvider {
	provider, ok := ctx.Value(providerCtxKey{}).(Text2AudioProvider)
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

func withOnStartInput(ctx context.Context, input *audios.CallbackInput) context.Context {
	return context.WithValue(ctx, onStartInputCtxKey{}, input)
}

func getOnStartInput(ctx context.Context) *audios.CallbackInput {
	v, ok := ctx.Value(onStartInputCtxKey{}).(*audios.CallbackInput)
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
