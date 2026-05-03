package impl

import (
	"context"
	"fmt"
	"strings"

	llmembedding "github.com/gonotelm-lab/gonotelm/internal/infra/llm/embedding"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/embedding/dashscope"
	"github.com/cloudwego/eino-ext/components/embedding/gemini"
	"github.com/cloudwego/eino-ext/components/embedding/ollama"
	"github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino-ext/components/embedding/qianfan"
	"github.com/cloudwego/eino-ext/components/embedding/tencentcloud"
	einoembedding "github.com/cloudwego/eino/components/embedding"
	"google.golang.org/genai"
)

func New(ctx context.Context, t llmembedding.Type, cfg *llmembedding.Config) (einoembedding.Embedder, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedding config must not be nil")
	}

	switch t {
	case llmembedding.Ark:
		arkCfg, err := buildArkConfig(cfg.Ark)
		if err != nil {
			return nil, err
		}
		return ark.NewEmbedder(ctx, arkCfg)
	case llmembedding.DashScope:
		return dashscope.NewEmbedder(ctx, &dashscope.EmbeddingConfig{
			APIKey:     cfg.DashScope.APIKey,
			Timeout:    cfg.DashScope.Timeout,
			Model:      cfg.DashScope.Model,
			Dimensions: cfg.DashScope.Dimensions,
		})
	case llmembedding.Gemini:
		return newGeminiEmbedder(ctx, cfg.Gemini)
	case llmembedding.Ollama:
		return ollama.NewEmbedder(ctx, &ollama.EmbeddingConfig{
			Timeout:   cfg.Ollama.Timeout,
			BaseURL:   cfg.Ollama.BaseURL,
			Model:     cfg.Ollama.Model,
			Truncate:  cfg.Ollama.Truncate,
			KeepAlive: cfg.Ollama.KeepAlive,
			Options:   cfg.Ollama.Options,
		})
	case llmembedding.OpenAI:
		openaiCfg, err := buildOpenAIConfig(cfg.OpenAI)
		if err != nil {
			return nil, err
		}
		return openai.NewEmbedder(ctx, openaiCfg)
	case llmembedding.Qianfan:
		return newQianfanEmbedder(ctx, cfg.Qianfan)
	case llmembedding.TencentCloud:
		return tencentcloud.NewEmbedder(ctx, &tencentcloud.EmbeddingConfig{
			SecretID:  cfg.TencentCloud.SecretID,
			SecretKey: cfg.TencentCloud.SecretKey,
			Region:    cfg.TencentCloud.Region,
		})
	default:
		return nil, fmt.Errorf("embedding type %q is not supported", t)
	}
}

func buildArkConfig(cfg llmembedding.ArkConfig) (*ark.EmbeddingConfig, error) {
	var apiType *ark.APIType
	if strings.TrimSpace(cfg.APIType) != "" {
		parsed, err := parseArkAPIType(cfg.APIType)
		if err != nil {
			return nil, err
		}
		apiType = &parsed
	}

	return &ark.EmbeddingConfig{
		Timeout:               cfg.Timeout,
		RetryTimes:            cfg.RetryTimes,
		BaseURL:               cfg.BaseURL,
		Region:                cfg.Region,
		APIKey:                cfg.APIKey,
		AccessKey:             cfg.AccessKey,
		SecretKey:             cfg.SecretKey,
		Model:                 cfg.Model,
		APIType:               apiType,
		MaxConcurrentRequests: cfg.MaxConcurrentRequests,
	}, nil
}

func parseArkAPIType(raw string) (ark.APIType, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(ark.APITypeText), "text":
		return ark.APITypeText, nil
	case string(ark.APITypeMultiModal), "multi_modal", "multimodal":
		return ark.APITypeMultiModal, nil
	default:
		return "", fmt.Errorf("unsupported ark api_type %q", raw)
	}
}

func newGeminiEmbedder(ctx context.Context, cfg llmembedding.GeminiConfig) (einoembedding.Embedder, error) {
	backend, err := parseGeminiBackend(cfg.Backend)
	if err != nil {
		return nil, err
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:   cfg.APIKey,
		Backend:  backend,
		Project:  cfg.Project,
		Location: cfg.Location,
	})
	if err != nil {
		return nil, fmt.Errorf("init gemini client failed: %w", err)
	}

	return gemini.NewEmbedder(ctx, &gemini.EmbeddingConfig{
		Client:               client,
		Model:                cfg.Model,
		TaskType:             cfg.TaskType,
		Title:                cfg.Title,
		OutputDimensionality: cfg.OutputDimensionality,
		MIMEType:             cfg.MIMEType,
		AutoTruncate:         cfg.AutoTruncate,
	})
}

func parseGeminiBackend(raw string) (genai.Backend, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "unspecified":
		return genai.BackendUnspecified, nil
	case "gemini", "gemini_api":
		return genai.BackendGeminiAPI, nil
	case "vertex", "vertex_ai":
		return genai.BackendVertexAI, nil
	case "enterprise":
		return genai.BackendVertexAI, nil
	default:
		return genai.BackendUnspecified, fmt.Errorf("unsupported gemini backend %q", raw)
	}
}

func buildOpenAIConfig(cfg llmembedding.OpenAIConfig) (*openai.EmbeddingConfig, error) {
	var (
		encodingFormat *openai.EmbeddingEncodingFormat
		userPtr        *string
	)

	if strings.TrimSpace(cfg.EncodingFormat) != "" {
		format, err := parseOpenAIEncodingFormat(cfg.EncodingFormat)
		if err != nil {
			return nil, err
		}
		encodingFormat = &format
	}

	if user := strings.TrimSpace(cfg.User); user != "" {
		userPtr = &user
	}

	return &openai.EmbeddingConfig{
		Timeout:        cfg.Timeout,
		APIKey:         cfg.APIKey,
		ByAzure:        cfg.ByAzure,
		BaseURL:        cfg.BaseURL,
		APIVersion:     cfg.APIVersion,
		Model:          cfg.Model,
		EncodingFormat: encodingFormat,
		Dimensions:     cfg.Dimensions,
		User:           userPtr,
	}, nil
}

func parseOpenAIEncodingFormat(raw string) (openai.EmbeddingEncodingFormat, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(openai.EmbeddingEncodingFormatFloat), "float32", "float64":
		return openai.EmbeddingEncodingFormatFloat, nil
	case string(openai.EmbeddingEncodingFormatBase64), "b64":
		return openai.EmbeddingEncodingFormatBase64, nil
	default:
		return "", fmt.Errorf("unsupported openai encoding_format %q", raw)
	}
}

func newQianfanEmbedder(ctx context.Context, cfg llmembedding.QianfanConfig) (einoembedding.Embedder, error) {
	qcfg := qianfan.GetQianfanSingletonConfig()

	if cfg.AK != "" {
		qcfg.AK = cfg.AK
	}
	if cfg.SK != "" {
		qcfg.SK = cfg.SK
	}
	if cfg.AccessKey != "" {
		qcfg.AccessKey = cfg.AccessKey
	}
	if cfg.SecretKey != "" {
		qcfg.SecretKey = cfg.SecretKey
	}
	if cfg.AccessToken != "" {
		qcfg.AccessToken = cfg.AccessToken
	}
	if cfg.BearerToken != "" {
		qcfg.BearerToken = cfg.BearerToken
	}

	return qianfan.NewEmbedder(ctx, &qianfan.EmbeddingConfig{
		Model:                 cfg.Model,
		LLMRetryCount:         cfg.LLMRetryCount,
		LLMRetryTimeout:       cfg.LLMRetryTimeout,
		LLMRetryBackoffFactor: cfg.LLMRetryBackoffFactor,
	})
}
