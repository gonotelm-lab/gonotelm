package text2image

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/multimodal/callbacks"
)

type Interceptor struct{}

func newInterceptor() callbacks.Handler {
	interceptor := &Interceptor{}
	return callbacks.NewHandlerBuilder().
		OnStartFn(interceptor.OnStart).
		OnEndFn(interceptor.OnEnd).
		OnErrorFn(interceptor.OnError).
		Build()
}

func (i *Interceptor) OnStart(
	ctx context.Context,
	info *callbacks.RunInfo,
	input callbacks.CallbackInput,
) context.Context {
	slog.DebugContext(ctx, "[text2image.Interceptor] OnStart", slog.Any("info", info))
	return ctx
}

func (i *Interceptor) OnEnd(
	ctx context.Context,
	info *callbacks.RunInfo,
	output callbacks.CallbackOutput,
) context.Context {
	slog.DebugContext(ctx, "[text2image.Interceptor] OnEnd", slog.Any("info", info))
	return ctx
}

func (i *Interceptor) OnError(
	ctx context.Context,
	info *callbacks.RunInfo,
	err error,
) context.Context {
	slog.ErrorContext(ctx, "[text2image.Interceptor] OnError",
		slog.Any("info", info),
		slog.Any("err", err))
	return ctx
}
