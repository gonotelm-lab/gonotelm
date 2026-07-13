package chat

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/safe"
)

func init() {
	pkgcontext.RegisterSlogAttrs(
		func(ctx context.Context) (slog.Attr, bool) {
			modelName := getModelName(ctx)
			if modelName == "" {
				return slog.Attr{}, false
			}

			return slog.String("model_name", modelName), true
		},
	)
}

type Interceptor struct{}

var _ callbacks.Handler = &Interceptor{}

func (i *Interceptor) OnStart(
	ctx context.Context,
	info *callbacks.RunInfo,
	input callbacks.CallbackInput,
) context.Context {
	return ctx
}

func (i *Interceptor) OnEnd(
	ctx context.Context,
	info *callbacks.RunInfo,
	output callbacks.CallbackOutput,
) context.Context {
	modelOutput := model.ConvCallbackOutput(output)
	if modelOutput == nil {
		slog.WarnContext(ctx, "[Interceptor] OnEnd empty callback output", slog.Any("info", info))
		return ctx
	}

	// attrs := getTokenUsageAttrs(modelOutput.TokenUsage)
	// attrs = append(attrs, slog.Any("info", info))
	// slog.DebugContext(ctx, "[Interceptor] OnEnd", attrs...)
	return ctx
}

func (i *Interceptor) OnError(
	ctx context.Context,
	info *callbacks.RunInfo,
	err error,
) context.Context {
	runSemRelease(ctx)

	slog.ErrorContext(ctx, "[Interceptor] OnError",
		slog.Any("info", info),
		slog.Bool("is_streaming", getIsStreaming(ctx)),
		slog.Any("err", err),
	)

	return ctx
}

func (i *Interceptor) OnStartWithStreamInput(
	ctx context.Context,
	info *callbacks.RunInfo,
	input *schema.StreamReader[callbacks.CallbackInput],
) context.Context {
	return ctx
}

func (i *Interceptor) OnEndWithStreamOutput(
	ctx context.Context,
	info *callbacks.RunInfo,
	output *schema.StreamReader[callbacks.CallbackOutput],
) context.Context {
	safe.Go(ctx, func() {
		defer func() {
			output.Close()
			runSemRelease(ctx)
		}()

		var lastCallbackOutput *model.CallbackOutput
		for {
			msg, err := output.Recv()
			modelOutput := model.ConvCallbackOutput(msg)
			if modelOutput != nil {
				lastCallbackOutput = modelOutput
			}

			if errors.Is(err, io.EOF) {
				if lastCallbackOutput == nil {
					slog.WarnContext(ctx, "[Interceptor] OnEndWithStreamOutput last callback output is nil",
						slog.Any("info", info))
					return
				}

				attrs := getTokenUsageAttrs(lastCallbackOutput.TokenUsage)
				attrs = append(attrs, slog.Any("info", info))
				slog.DebugContext(ctx, "[Interceptor] OnEndWithStreamOutput", attrs...)
				break
			}

			if err != nil {
				slog.ErrorContext(ctx, "[Interceptor] OnEndWithStreamOutput Recv error",
					slog.Any("info", info),
					slog.Any("err", err),
				)
				break
			}
		}
	})

	return ctx
}

func getTokenUsageAttrs(
	tokenUsage *model.TokenUsage,
) []any {
	return []any{
		slog.Int("completion_tokens", tokenUsage.CompletionTokens),
		slog.Int("prompt_tokens", tokenUsage.PromptTokens),
		slog.Int("cached_tokens", tokenUsage.PromptTokenDetails.CachedTokens),
		slog.Int("total_tokens", tokenUsage.TotalTokens),
	}
}
