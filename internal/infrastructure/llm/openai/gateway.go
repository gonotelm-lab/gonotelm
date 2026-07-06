package openai

import (
	"context"
	"fmt"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"

	deepseekext "github.com/cloudwego/eino-ext/components/model/deepseek"
	qwenext "github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
	"golang.org/x/sync/semaphore"
)

const (
	wrappedChatModelRunName = "gateway-chat-model"

	defaultMaxConcurrency = 250
)

type Gateway struct {
	mu sync.RWMutex

	providers map[llm.Provider]einomodel.ToolCallingChatModel
}

func New(cfg *llm.ProviderConfig) (*Gateway, error) {
	gw := &Gateway{
		providers: make(map[llm.Provider]einomodel.ToolCallingChatModel),
	}

	err := gw.initProviders(cfg)
	if err != nil {
		return nil, err
	}

	return gw, nil
}

func (g *Gateway) initProviders(cfg *llm.ProviderConfig) error {
	ctx := context.Background()
	deepseekModel, err := NewChatModel(ctx, llm.ProviderDeepSeek, cfg)
	if err != nil {
		return err
	}
	g.providers[llm.ProviderDeepSeek] = newWrappedChatModel(deepseekModel, llm.ProviderDeepSeek, cfg.DeepSeek.MaxConcurrency)

	openaiModel, err := NewChatModel(ctx, llm.ProviderOpenAI, cfg)
	if err != nil {
		return err
	}
	g.providers[llm.ProviderOpenAI] = newWrappedChatModel(openaiModel, llm.ProviderOpenAI, cfg.OpenAI.MaxConcurrency)

	qwenModel, err := NewChatModel(ctx, llm.ProviderQwen, cfg)
	if err != nil {
		return err
	}
	g.providers[llm.ProviderQwen] = newWrappedChatModel(qwenModel, llm.ProviderQwen, cfg.Qwen.MaxConcurrency)

	agnesModel, err := NewChatModel(ctx, llm.ProviderAgnes, cfg)
	if err != nil {
		return err
	}
	g.providers[llm.ProviderAgnes] = newWrappedChatModel(agnesModel, llm.ProviderAgnes, cfg.Agnes.MaxConcurrency)

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
	typ            string
	provider       llm.Provider
	impl           einomodel.ToolCallingChatModel
	maxConcurrency int
	sem            *semaphore.Weighted
}

func newWrappedChatModel(
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
	}, &Interceptor{})

	err := g.sem.Acquire(ctx, 1)
	if err != nil {
		return nil, err
	}
	defer g.sem.Release(1)

	return g.impl.Generate(ctx, input, opts...)
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
	}, &Interceptor{})

	switch g.provider {
	case llm.ProviderDeepSeek:
		opts = append(opts, deepseekext.WithExtraFields(streamOptionsIncludeUsage))
	case llm.ProviderQwen:
		opts = append(opts, qwenext.WithExtraFields(streamOptionsIncludeUsage))
	}

	err := g.sem.Acquire(ctx, 1)
	if err != nil {
		return nil, err
	}
	releaseSem := sync.OnceFunc(func() {
		g.sem.Release(1)
	})
	ctx = withSemReleaseFunc(ctx, releaseSem)

	stream, err := g.impl.Stream(ctx, input, opts...)
	if err != nil {
		releaseSem()
		return nil, err
	}

	return stream, nil
}

func (g *wrappedChatModel) WithTools(
	tools []*einoschema.ToolInfo,
) (einomodel.ToolCallingChatModel, error) {
	impl, err := g.impl.WithTools(tools)
	if err != nil {
		return nil, err
	}

	return newWrappedChatModel(impl, g.provider, g.maxConcurrency), nil
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
