package embedding

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/embedding/cache"
	embedredis "github.com/cloudwego/eino-ext/components/embedding/cache/redis"
	"github.com/cloudwego/eino-ext/components/embedding/dashscope"
	"github.com/cloudwego/eino-ext/components/embedding/gemini"
	"github.com/cloudwego/eino-ext/components/embedding/ollama"
	"github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino-ext/components/embedding/qianfan"
	"github.com/cloudwego/eino-ext/components/embedding/tencentcloud"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/redis/go-redis/v9"
	"google.golang.org/genai"
)

func New(
	ctx context.Context,
	cfg *Config,
	cacher cache.Cacher,
) (embedding.Embedder, error) {
	if cfg == nil {
		return nil, fmt.Errorf("embedding config must not be nil")
	}

	var (
		embedder embedding.Embedder
		err      error
	)

	switch cfg.Type {
	case Ark:
		var arkCfg *ark.EmbeddingConfig
		arkCfg, err = buildArkConfig(cfg.Ark)
		if err != nil {
			return nil, err
		}
		embedder, err = ark.NewEmbedder(ctx, arkCfg)
	case DashScope:
		embedder, err = dashscope.NewEmbedder(ctx, &dashscope.EmbeddingConfig{
			APIKey:     cfg.DashScope.APIKey,
			Timeout:    cfg.DashScope.Timeout,
			Model:      cfg.DashScope.Model,
			Dimensions: cfg.DashScope.Dimensions,
		})
	case Gemini:
		embedder, err = newGeminiEmbedder(ctx, cfg.Gemini)
	case Ollama:
		embedder, err = ollama.NewEmbedder(ctx, &ollama.EmbeddingConfig{
			Timeout:   cfg.Ollama.Timeout,
			BaseURL:   cfg.Ollama.BaseURL,
			Model:     cfg.Ollama.Model,
			Truncate:  cfg.Ollama.Truncate,
			KeepAlive: cfg.Ollama.KeepAlive,
			Options:   cfg.Ollama.Options,
		})
	case OpenAI:
		var openaiCfg *openai.EmbeddingConfig
		openaiCfg, err = buildOpenAIConfig(cfg.OpenAI)
		if err != nil {
			return nil, err
		}
		embedder, err = openai.NewEmbedder(ctx, openaiCfg)
	case Qianfan:
		embedder, err = newQianfanEmbedder(ctx, cfg.Qianfan)
	case TencentCloud:
		embedder, err = tencentcloud.NewEmbedder(ctx, &tencentcloud.EmbeddingConfig{
			SecretID:  cfg.TencentCloud.SecretID,
			SecretKey: cfg.TencentCloud.SecretKey,
			Region:    cfg.TencentCloud.Region,
		})
	default:
		err = fmt.Errorf("type %q is not supported", cfg.Type)
	}
	if err != nil {
		return nil, err
	}

	if cacher != nil {
		embedder, err = cache.NewEmbedder(embedder,
			cache.WithCacher(cacher),
			cache.WithExpiration(0), // TODO never expire
			cache.WithGenerator(newEmbedKeyGenerator()),
		)
		if err != nil {
			return nil, err
		}
	}

	return embedder, nil
}

func buildArkConfig(cfg ArkConfig) (*ark.EmbeddingConfig, error) {
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

func newGeminiEmbedder(ctx context.Context, cfg GeminiConfig) (embedding.Embedder, error) {
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

func buildOpenAIConfig(cfg OpenAIConfig) (*openai.EmbeddingConfig, error) {
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

func newQianfanEmbedder(ctx context.Context, cfg QianfanConfig) (embedding.Embedder, error) {
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

func NewRedisCacher(rdb redis.UniversalClient) cache.Cacher {
	return embedredis.NewCacher(rdb, embedredis.WithPrefix("gonotelm:embed:"))
}
