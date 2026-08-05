package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	"github.com/gonotelm-lab/gonotelm/pkg/safe"
	pkgtrace "github.com/gonotelm-lab/gonotelm/pkg/trace"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/instrumentation/genaiconv"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/semaphore"
)

const (
	wrappedChatModelRunName = "gateway-chat-model"

	defaultMaxConcurrency = 250
)

type Gateway struct {
	rootCtx context.Context
	mu      sync.RWMutex

	providers map[llm.Provider]einomodel.ToolCallingChatModel
}

func New(ctx context.Context, cfg *llm.ProviderConfig) (*Gateway, error) {
	gw := &Gateway{
		rootCtx:   ctx,
		providers: make(map[llm.Provider]einomodel.ToolCallingChatModel),
	}

	err := gw.initProviders(cfg)
	if err != nil {
		return nil, err
	}

	return gw, nil
}

func (g *Gateway) initProviders(cfg *llm.ProviderConfig) error {
	deepseekModel, err := NewChatModel(g.rootCtx, llm.ProviderDeepSeek, cfg)
	if err != nil {
		return err
	}
	g.providers[llm.ProviderDeepSeek] = newWrappedChatModel(g.rootCtx, deepseekModel, llm.ProviderDeepSeek, cfg.DeepSeek.MaxConcurrency)

	openaiModel, err := NewChatModel(g.rootCtx, llm.ProviderOpenAI, cfg)
	if err != nil {
		return err
	}
	g.providers[llm.ProviderOpenAI] = newWrappedChatModel(g.rootCtx, openaiModel, llm.ProviderOpenAI, cfg.OpenAI.MaxConcurrency)

	qwenModel, err := NewChatModel(g.rootCtx, llm.ProviderQwen, cfg)
	if err != nil {
		return err
	}
	g.providers[llm.ProviderQwen] = newWrappedChatModel(g.rootCtx, qwenModel, llm.ProviderQwen, cfg.Qwen.MaxConcurrency)

	agnesModel, err := NewChatModel(g.rootCtx, llm.ProviderAgnes, cfg)
	if err != nil {
		return err
	}
	g.providers[llm.ProviderAgnes] = newWrappedChatModel(g.rootCtx, agnesModel, llm.ProviderAgnes, cfg.Agnes.MaxConcurrency)

	return nil
}

func (g *Gateway) GetProvider(providerType llm.Provider) (einomodel.ToolCallingChatModel, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	provider, ok := g.providers[providerType]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", providerType)
	}

	return provider, nil
}

type wrappedChatModel struct {
	rootCtx        context.Context
	typ            string
	provider       llm.Provider
	impl           einomodel.ToolCallingChatModel
	maxConcurrency int
	sem            *semaphore.Weighted
}

func newWrappedChatModel(
	ctx context.Context,
	impl einomodel.ToolCallingChatModel,
	provider llm.Provider,
	maxConcurrency int,
) *wrappedChatModel {
	typ, ok := components.GetType(impl)
	if !ok {
		typ = "GatewayWrapped"
	}

	if maxConcurrency <= 0 {
		maxConcurrency = defaultMaxConcurrency
	}

	sem := semaphore.NewWeighted(int64(maxConcurrency))

	return &wrappedChatModel{
		rootCtx:        ctx,
		typ:            typ,
		provider:       provider,
		impl:           impl,
		maxConcurrency: maxConcurrency,
		sem:            sem,
	}
}

var _ einomodel.ToolCallingChatModel = &wrappedChatModel{}

func (g *wrappedChatModel) Generate(
	ctx context.Context,
	input []*einoschema.Message,
	opts ...einomodel.Option,
) (*einoschema.Message, error) {
	modelName := extractOptionModelName(opts...)
	ctx = withModelName(ctx, modelName)
	ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{
		Name:      wrappedChatModelRunName,
		Type:      g.typ,
		Component: components.ComponentOfChatModel,
	}, &Interceptor{rootCtx: g.rootCtx})

	opts = applyProviderCallOptions(g.provider, false, opts)

	err := g.sem.Acquire(ctx, 1)
	if err != nil {
		return nil, err
	}
	defer g.sem.Release(1)

	ctx, span := startChatSpan(ctx, g.provider, modelName)
	defer span.End()

	resp, err := g.impl.Generate(ctx, input, opts...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return resp, err
}

func (g *wrappedChatModel) Stream(
	ctx context.Context,
	input []*einoschema.Message,
	opts ...einomodel.Option,
) (*einoschema.StreamReader[*einoschema.Message], error) {
	modelName := extractOptionModelName(opts...)
	ctx = withModelName(ctx, modelName)
	ctx = withIsStreaming(ctx, true)
	ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{
		Name:      wrappedChatModelRunName,
		Type:      g.typ,
		Component: components.ComponentOfChatModel,
	}, &Interceptor{rootCtx: g.rootCtx})

	opts = applyProviderCallOptions(g.provider, true, opts)

	err := g.sem.Acquire(ctx, 1)
	if err != nil {
		return nil, err
	}
	releaseSem := sync.OnceFunc(func() {
		g.sem.Release(1)
	})
	ctx = withSemReleaseFunc(ctx, releaseSem)

	ctx, span := startChatSpan(ctx, g.provider, modelName)

	stream, err := g.impl.Stream(ctx, input, opts...)
	if err != nil {
		releaseSem()
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.End()
		return nil, err
	}

	// span 在流式输出消费完（EOF/错误/Close）时结束：
	// 通过 pipe 转发，后台 goroutine 在流结束时结束 span。
	reader, writer := einoschema.Pipe[*einoschema.Message](0)
	safe.DetachGo(ctx, g.rootCtx, "chat.stream_trace", func(ctx context.Context) {
		defer writer.Close()
		defer span.End()

		for {
			msg, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return
			}
			if closed := writer.Send(msg, nil); closed {
				// 消费者提前关闭，正常结束，不标记错误
				return
			}
		}
	})

	return reader, nil
}

// startChatSpan 为一次 chat 调用创建 gen_ai client span。
func startChatSpan(ctx context.Context, provider llm.Provider, modelName string) (context.Context, oteltrace.Span) {
	return pkgtrace.GetOtelTracer().Start(ctx, genaiconv.SpanName("chat"),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(genaiconv.Attributes(string(provider), "chat", modelName)...),
	)
}

func (g *wrappedChatModel) WithTools(
	tools []*einoschema.ToolInfo,
) (einomodel.ToolCallingChatModel, error) {
	impl, err := g.impl.WithTools(tools)
	if err != nil {
		return nil, err
	}

	return newWrappedChatModel(g.rootCtx, impl, g.provider, g.maxConcurrency), nil
}

func extractOptionModelName(opts ...einomodel.Option) string {
	option := einomodel.GetCommonOptions(&einomodel.Options{}, opts...)
	if option.Model == nil {
		return ""
	}
	modelName := *option.Model
	if modelName == "" {
		return ""
	}

	return modelName
}
