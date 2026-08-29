package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/util"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/safe"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
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
	rootCtx  context.Context
	recorder Recorder
}

func newInterceptor(rootCtx context.Context, recorder Recorder) *Interceptor {
	return &Interceptor{rootCtx: rootCtx, recorder: recorder}
}

// Docs see: https://www.cloudwego.io/zh/docs/eino/quick_start/chapter_06_callback_and_trace/
var _ callbacks.Handler = &Interceptor{}

// non-streaming and streaming start callback
func (i *Interceptor) OnStart(
	ctx context.Context,
	info *callbacks.RunInfo,
	src callbacks.CallbackInput,
) context.Context {
	slog.DebugContext(ctx, "[chat.Interceptor] OnStart", slog.Any("info", util.RenameRunInfo(info)))
	input := model.ConvCallbackInput(src)
	start := time.Now()
	ctx = withOnStartInput(ctx, input)

	return withStartTime(ctx, start)
}

func (i *Interceptor) OnEnd(
	ctx context.Context,
	info *callbacks.RunInfo,
	src callbacks.CallbackOutput,
) context.Context {
	slog.DebugContext(ctx, "[chat.Interceptor] OnEnd", slog.Any("info", util.RenameRunInfo(info)))
	output := model.ConvCallbackOutput(src)
	if output == nil {
		slog.WarnContext(ctx, "[chat.Interceptor] OnEnd empty callback output", slog.Any("info", util.RenameRunInfo(info)))
		return ctx
	}

	i.recordEnd(ctx, output)
	return ctx
}

// non-streaming or streaming error callback
// 对于流式输出 OnError在创建流式输出前出错时会被回调 流式输出过程中错误不会被调用
func (i *Interceptor) OnError(
	ctx context.Context,
	info *callbacks.RunInfo,
	err error,
) context.Context {
	runSemRelease(ctx)

	slog.ErrorContext(ctx, "[chat.Interceptor] OnError",
		slog.Any("info", util.RenameRunInfo(info)), slog.Bool("streaming", getStreaming(ctx)), slog.Any("err", err),
	)

	i.recordError(ctx, err)
	return ctx
}

func (i *Interceptor) OnEndWithStreamOutput(
	ctx context.Context,
	info *callbacks.RunInfo,
	output *schema.StreamReader[callbacks.CallbackOutput],
) context.Context {
	// 流式触发 需要后台协程单独处理
	safe.DetachGo(ctx, i.rootCtx, "chat.stream_output", func(ctx context.Context) {
		defer func() {
			output.Close() // we must call this ourselves to prevent goroutine leaks
			runSemRelease(ctx)
		}()

		var accumulatedChunks []*model.CallbackOutput

		for {
			callbackOutput, err := output.Recv()
			modelOutputChunk := model.ConvCallbackOutput(callbackOutput)
			if modelOutputChunk != nil {
				accumulatedChunks = append(accumulatedChunks, modelOutputChunk)
			}

			if errors.Is(err, io.EOF) {
				if len(accumulatedChunks) == 0 {
					i.recordError(ctx, fmt.Errorf("interceptor accumulated chunks are empty"))
					return
				}

				// now we concat the final message out of streamed chunks
				chunkMsgs := make([]*schema.Message, 0, len(accumulatedChunks))
				for _, chunk := range accumulatedChunks {
					chunkMsgs = append(chunkMsgs, chunk.Message)
				}
				finalMsg, err := schema.ConcatMessages(chunkMsgs)
				if err != nil {
					i.recordError(ctx, err)
					break
				}

				lastChunk := accumulatedChunks[len(accumulatedChunks)-1]
				lastChunk.Message = finalMsg

				i.recordEnd(ctx, lastChunk)
				break
			}

			if err != nil {
				slog.ErrorContext(ctx, "[chat.Interceptor] OnEndWithStreamOutput Recv error",
					slog.Any("info", util.RenameRunInfo(info)),
					slog.Any("err", err),
				)

				// 流式输出过程中出错此处处理
				i.recordError(ctx, err)
				break
			}
		}
	})

	return ctx
}

func (i *Interceptor) recordError(ctx context.Context, err error) {
	if i.recorder != nil {
		if rErr := i.recorder.Record(ctx, buildErrorRecord(ctx, err, time.Now())); rErr != nil {
			slog.ErrorContext(ctx, "[chat.Interceptor] record error failed", slog.Any("err", rErr))
		}
	}
}

func (i *Interceptor) recordEnd(ctx context.Context, output *model.CallbackOutput) {
	if i.recorder != nil {
		r := buildEndRecord(ctx, getOnStartInput(ctx), output, time.Now())
		if err := i.recorder.Record(ctx, r); err != nil {
			slog.ErrorContext(ctx, "[chat.Interceptor] record end failed", slog.Any("err", err))
		}
	}
}

// useless for llm cases, only for callbacks.Handler interface implementation
func (i *Interceptor) OnStartWithStreamInput(
	ctx context.Context,
	info *callbacks.RunInfo,
	input *schema.StreamReader[callbacks.CallbackInput],
) context.Context {
	return ctx
}
