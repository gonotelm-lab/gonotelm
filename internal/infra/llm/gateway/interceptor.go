package gateway

import (
	"context"
	"log/slog"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
)

// ```
// ┌─────────────────────────────────────────┐
// │  ChatModel.Generate(ctx, messages)      │
// └─────────────────────────────────────────┘
//
//	           ↓
//	┌──────────────────────┐
//	│  OnStart             │  ← 输入: CallbackInput (messages)
//	└──────────────────────┘
//	           ↓
//	┌──────────────────────┐
//	│  模型处理             │
//	└──────────────────────┘
//	           ↓
//	┌──────────────────────┐
//	│  OnEnd               │  ← 输出: CallbackOutput (response)
//	└──────────────────────┘
//
// ```
// ---
// ```
// ┌─────────────────────────────────────────┐
// │  ChatModel.Stream(ctx, messages)        │
// └─────────────────────────────────────────┘
//
//	           ↓
//	┌──────────────────────┐
//	│  OnStart             │  ← 输入: CallbackInput (messages)
//	└──────────────────────┘
//	           ↓
//	┌──────────────────────┐
//	│  模型处理（流式）     │
//	└──────────────────────┘
//	           ↓
//	┌──────────────────────┐
//	│  OnEndWithStreamOutput │  ← 输出: StreamReader[CallbackOutput]
//	└──────────────────────┘
//	           ↓
//	┌──────────────────────┐
//	│  逐个 chunk 返回      │
//	└──────────────────────┘
//
// ```
type Interceptor struct{}

var _ callbacks.Handler = &Interceptor{}

func (i *Interceptor) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	modelName := getModelName(ctx)
	slog.DebugContext(ctx, "[Interceptor] OnStart", slog.Any("info", info), slog.String("modelName", modelName))
	return ctx
}

func (i *Interceptor) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	modelName := getModelName(ctx)
	slog.DebugContext(ctx, "[Interceptor] OnEnd", slog.Any("info", info), slog.String("modelName", modelName))
	return ctx
}

func (i *Interceptor) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	modelName := getModelName(ctx)
	slog.ErrorContext(ctx, "[Interceptor] OnError", slog.Any("info", info), slog.String("modelName", modelName), slog.Any("err", err))
	return ctx
}

func (i *Interceptor) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo,
	input *schema.StreamReader[callbacks.CallbackInput],
) context.Context {
	slog.DebugContext(ctx, "[Interceptor] OnStartWithStreamInput", slog.Any("info", info))
	return ctx
}

func (i *Interceptor) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo,
	output *schema.StreamReader[callbacks.CallbackOutput],
) context.Context {
	// TODO 需要流式消费output中的callback
	return ctx
}
