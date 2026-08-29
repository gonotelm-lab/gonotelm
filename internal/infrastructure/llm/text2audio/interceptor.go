package text2audio

import (
	"context"
	"log/slog"
	"time"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	audios "github.com/gonotelm-lab/multimodal/audio"
	"github.com/gonotelm-lab/multimodal/callbacks"
)

func init() {
	pkgcontext.RegisterSlogAttrs(
		func(ctx context.Context) (slog.Attr, bool) {
			modelName := getModelName(ctx)
			if modelName == "" {
				return slog.Attr{}, false
			}

			return slog.String("text2audio.model", modelName), true
		},
	)
}

type Interceptor struct {
	recorder Recorder
}

func newInterceptor(recorder Recorder) callbacks.Handler {
	interceptor := &Interceptor{recorder: recorder}
	return callbacks.NewHandlerBuilder().
		OnStartFn(interceptor.OnStart).
		OnEndFn(interceptor.OnEnd).
		OnErrorFn(interceptor.OnError).
		Build()
}

func (i *Interceptor) OnStart(
	ctx context.Context,
	info *callbacks.RunInfo,
	src callbacks.CallbackInput,
) context.Context {
	slog.DebugContext(ctx, "[text2audio.Interceptor] OnStart", slog.Any("info", info))
	input := audios.ConvCallbackInput(src)
	if input == nil {
		slog.WarnContext(ctx, "[text2audio.Interceptor] OnStart got empty input")
		return ctx
	}

	start := time.Now()
	ctx = withOnStartInput(ctx, input)
	return withStartTime(ctx, start)
}

func (i *Interceptor) OnEnd(
	ctx context.Context,
	info *callbacks.RunInfo,
	src callbacks.CallbackOutput,
) context.Context {
	slog.DebugContext(ctx, "[text2audio.Interceptor] OnEnd", slog.Any("info", info))
	output := audios.ConvCallbackOutput(src)
	if output == nil {
		slog.WarnContext(ctx, "[text2audio.Interceptor] OnEnd got empty output")
		return ctx
	}

	i.recordEnd(ctx, output)
	return ctx
}

func (i *Interceptor) OnError(
	ctx context.Context,
	info *callbacks.RunInfo,
	err error,
) context.Context {
	slog.ErrorContext(ctx, "[text2audio.Interceptor] OnError",
		slog.Any("info", info),
		slog.Any("err", err))

	i.recordError(ctx, err)
	return ctx
}

func (i *Interceptor) recordError(ctx context.Context, err error) {
	if i.recorder == nil {
		return
	}
	if rErr := i.recorder.Record(ctx, buildErrorRecord(ctx, err, time.Now())); rErr != nil {
		slog.ErrorContext(ctx, "[text2audio.Interceptor] record error failed", slog.Any("err", rErr))
	}
}

func (i *Interceptor) recordEnd(ctx context.Context, output *audios.CallbackOutput) {
	if i.recorder == nil {
		return
	}
	r := buildEndRecord(ctx, getOnStartInput(ctx), output, time.Now())
	if err := i.recorder.Record(ctx, r); err != nil {
		slog.ErrorContext(ctx, "[text2audio.Interceptor] record end failed", slog.Any("err", err))
	}
}
