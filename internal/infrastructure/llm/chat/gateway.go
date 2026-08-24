package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

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

	providers map[Provider]einomodel.ToolCallingChatModel

	recorder Recorder
}

type gatewayOption struct {
	recorder Recorder
}

type GatewayOption func(o *gatewayOption)

func WithRecorder(r Recorder) GatewayOption {
	return func(o *gatewayOption) {
		o.recorder = r
	}
}

func New(ctx context.Context, cfg *ProviderConfig, opts ...GatewayOption) (*Gateway, error) {
	opt := gatewayOption{}
	for _, o := range opts {
		if o != nil {
			o(&opt)
		}
	}

	g := &Gateway{
		rootCtx:   ctx,
		providers: make(map[Provider]einomodel.ToolCallingChatModel),
		recorder:  opt.recorder,
	}

	err := g.initProviders(cfg)
	if err != nil {
		return nil, err
	}

	return g, nil
}

func (g *Gateway) initProviders(cfg *ProviderConfig) error {
	deepseekModel, err := NewChatModel(g.rootCtx, ProviderDeepSeek, cfg)
	if err != nil {
		return err
	}
	g.providers[ProviderDeepSeek] = newWrappedChatModel(g.rootCtx, deepseekModel, ProviderDeepSeek,
		cfg.DeepSeek.MaxConcurrency, g.recorder,
	)

	openaiModel, err := NewChatModel(g.rootCtx, ProviderOpenAI, cfg)
	if err != nil {
		return err
	}
	g.providers[ProviderOpenAI] = newWrappedChatModel(g.rootCtx, openaiModel, ProviderOpenAI,
		cfg.OpenAI.MaxConcurrency, g.recorder,
	)

	qwenModel, err := NewChatModel(g.rootCtx, ProviderQwen, cfg)
	if err != nil {
		return err
	}
	g.providers[ProviderQwen] = newWrappedChatModel(g.rootCtx, qwenModel, ProviderQwen,
		cfg.Qwen.MaxConcurrency, g.recorder,
	)

	agnesModel, err := NewChatModel(g.rootCtx, ProviderAgnes, cfg)
	if err != nil {
		return err
	}
	g.providers[ProviderAgnes] = newWrappedChatModel(g.rootCtx, agnesModel, ProviderAgnes,
		cfg.Agnes.MaxConcurrency, g.recorder,
	)

	return nil
}

func (g *Gateway) GetProvider(providerType Provider) (einomodel.ToolCallingChatModel, error) {
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
	provider       Provider
	impl           einomodel.ToolCallingChatModel
	maxConcurrency int
	sem            *semaphore.Weighted
	recorder       Recorder
}

func newWrappedChatModel(
	ctx context.Context,
	impl einomodel.ToolCallingChatModel,
	provider Provider,
	maxConcurrency int,
	recorder Recorder,
) *wrappedChatModel {
	typ, ok := components.GetType(impl)
	if !ok {
		typ = "WrappedGateway"
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
		recorder:       recorder,
	}
}

var _ einomodel.ToolCallingChatModel = &wrappedChatModel{}

func (g *wrappedChatModel) Generate(
	ctx context.Context,
	input []*einoschema.Message,
	opts ...einomodel.Option,
) (*einoschema.Message, error) {
	modelName := extractOptionModelName(opts...)
	opts, callOpts := applyProviderCallOptions(g.provider, false, opts)
	ctx = withModelName(ctx, modelName)
	ctx = withProvider(ctx, g.provider)
	ctx = withCallOptions(ctx, callOpts)
	ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{
		Name:      wrappedChatModelRunName,
		Type:      g.typ,
		Component: components.ComponentOfChatModel,
	}, newInterceptor(g.rootCtx, g.recorder))

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
	opts, callOpts := applyProviderCallOptions(g.provider, true, opts)
	ctx = withModelName(ctx, modelName)
	ctx = withStreaming(ctx, true)
	ctx = withProvider(ctx, g.provider)
	ctx = withCallOptions(ctx, callOpts)
	ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{
		Name:      wrappedChatModelRunName,
		Type:      g.typ,
		Component: components.ComponentOfChatModel,
	}, newInterceptor(g.rootCtx, g.recorder))

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
func startChatSpan(ctx context.Context, provider Provider, modelName string) (context.Context, oteltrace.Span) {
	return pkgtrace.GetOtelTracer().
		Start(
			ctx,
			genaiconv.SpanName("chat"), // gen_ai.chat
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

	return newWrappedChatModel(g.rootCtx, impl, g.provider, g.maxConcurrency, g.recorder), nil
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
