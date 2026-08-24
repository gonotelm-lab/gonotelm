package embedding

import (
	"context"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/util"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/embedding"
)

func init() {
	pkgcontext.RegisterSlogAttrs(
		func(ctx context.Context) (slog.Attr, bool) {
			modelName := getModelName(ctx)
			if modelName == "" {
				return slog.Attr{}, false
			}

			return slog.String("llm.model", modelName), true
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
	slog.DebugContext(ctx, "[embedding.Interceptor] OnStart", slog.Any("info", util.RenameRunInfo(info)))
	input := embedding.ConvCallbackInput(src)
	if input == nil {
		slog.WarnContext(ctx, "[embedding.Interceptor] OnStart got empty input")
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
	slog.DebugContext(ctx, "[embedding.Interceptor] OnEnd", slog.Any("info", util.RenameRunInfo(info)))
	output := embedding.ConvCallbackOutput(src)
	if output == nil {
		slog.WarnContext(ctx, "[embedding.Interceptor] OnEnd got empty output")
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
	slog.ErrorContext(ctx, "[embedding.Interceptor] OnError",
		slog.Any("err", err),
		slog.Any("info", util.RenameRunInfo(info)))

	i.recordError(ctx, err)
	return ctx
}

func (i *Interceptor) recordError(ctx context.Context, err error) {
	if i.recorder == nil {
		return
	}
	if rErr := i.recorder.Record(ctx, buildErrorRecord(ctx, err, time.Now())); rErr != nil {
		slog.ErrorContext(ctx, "[embedding.Interceptor] record error failed", slog.Any("err", rErr))
	}
}

func (i *Interceptor) recordEnd(ctx context.Context, output *embedding.CallbackOutput) {
	if i.recorder == nil {
		return
	}
	r := buildEndRecord(ctx, getOnStartInput(ctx), output, time.Now())
	if err := i.recorder.Record(ctx, r); err != nil {
		slog.ErrorContext(ctx, "[embedding.Interceptor] record end failed", slog.Any("err", err))
	}
}
