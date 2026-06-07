package gateway

import (
	"context"
	"fmt"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
)

const (
	wrappedChatModelRunName = "gateway-chat-model"
)

// 管理项目中的多个提供商的LLM模型，根据配置选择不同的模型
//
// TODO 可以在此Gateway中统一管控模型：比如模型动态增减、token消耗记数、监控追踪等
type Gateway struct {
	mu sync.RWMutex

	providers map[chat.Provider]einomodel.ToolCallingChatModel
}

func New(cfg *chat.ProviderConfig) (*Gateway, error) {
	gw := &Gateway{
		providers: make(map[chat.Provider]einomodel.ToolCallingChatModel),
	}

	// 初始化模型实例
	err := gw.initProviders(cfg)
	if err != nil {
		return nil, err
	}

	return gw, nil
}

func (g *Gateway) initProviders(cfg *chat.ProviderConfig) error {
	ctx := context.Background()
	// 1. deepseek
	deepseekModel, err := chat.New(ctx, chat.DeepSeek, cfg)
	if err != nil {
		return err
	}
	g.providers[chat.DeepSeek] = newWrappedChatModel(deepseekModel)

	// 2. openai
	openaiModel, err := chat.New(ctx, chat.Openai, cfg)
	if err != nil {
		return err
	}
	g.providers[chat.Openai] = newWrappedChatModel(openaiModel)

	// 3. qwen
	qwenModel, err := chat.New(ctx, chat.Qwen, cfg)
	if err != nil {
		return err
	}
	g.providers[chat.Qwen] = newWrappedChatModel(qwenModel)

	return nil
}

func (g *Gateway) GetProvider(
	providerType chat.Provider,
) (einomodel.ToolCallingChatModel, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	provider, ok := g.providers[providerType]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", providerType)
	}

	return provider, nil
}

type wrappedChatModel struct {
	typ  string
	impl einomodel.ToolCallingChatModel
}

func newWrappedChatModel(impl einomodel.ToolCallingChatModel) *wrappedChatModel {
	typ, ok := components.GetType(impl)
	if !ok {
		typ = "GatewayWrapped"
	}
	return &wrappedChatModel{
		typ:  typ,
		impl: impl,
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

	return g.impl.Generate(ctx, input, opts...)
}

func (g *wrappedChatModel) Stream(
	ctx context.Context,
	input []*einoschema.Message,
	opts ...einomodel.Option,
) (*einoschema.StreamReader[*einoschema.Message], error) {
	modelName := extractOptionModelName(opts...)
	ctx = withModelName(ctx, modelName)
	ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{
		Name:      wrappedChatModelRunName,
		Type:      g.typ,
		Component: components.ComponentOfChatModel,
	}, &Interceptor{})

	return g.impl.Stream(ctx, input, opts...)
}

func (g *wrappedChatModel) WithTools(
	tools []*einoschema.ToolInfo,
) (einomodel.ToolCallingChatModel, error) {
	impl, err := g.impl.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &wrappedChatModel{impl: impl}, nil
}

func extractOptionModelName(opts ...einomodel.Option) string {
	option := einomodel.GetCommonOptions(&einomodel.Options{}, opts...)
	if option.Model == nil {
		return "unspecified"
	}
	modelName := *option.Model
	if modelName == "" {
		return "unspecified"
	}

	return modelName
}

type modelNameKeyType struct{}

func withModelName(ctx context.Context, modelName string) context.Context {
	return context.WithValue(ctx, modelNameKeyType{}, modelName)
}

func getModelName(ctx context.Context) string {
	modelName, ok := ctx.Value(modelNameKeyType{}).(string)
	if !ok {
		return "unspecified"
	}
	return modelName
}
