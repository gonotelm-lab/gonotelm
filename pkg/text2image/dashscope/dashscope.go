package dashscope

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gonotelm-lab/gonotelm/pkg/text2image"
	"github.com/gonotelm-lab/gonotelm/pkg/text2image/schema"
)

// https://help.aliyun.com/zh/model-studio/qwen-image-api

const (
	defaultBaseUrl = "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
	defaultModel   = "qwen-image-2.0-pro"
)

type Generator struct {
	cfg        Config
	httpClient *http.Client
}

func New(cfg Config, opts ...text2image.ClientOption) (*Generator, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("dashscope api key is required")
	}
	if strings.TrimSpace(cfg.BaseUrl) == "" {
		cfg.BaseUrl = defaultBaseUrl
	}
	if strings.TrimSpace(cfg.Model) == "" {
		cfg.Model = defaultModel
	}

	co := text2image.BuildClientOptions(cfg.Timeout, opts...)
	return &Generator{
		cfg:        cfg,
		httpClient: co.HTTPClient,
	}, nil
}

func (g *Generator) Generate(ctx context.Context, req *schema.Request, opts ...text2image.Option) (*schema.Response, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	callOpts := text2image.BuildCallOptions(opts...)

	// 构建 parameters
	parameters := buildParameters(req, callOpts)

	// 模型优先级：Option > Request > Config
	model := g.cfg.Model
	if req.Model != "" {
		model = req.Model
	}

	// 构建请求体
	payload := apiRequest{
		Model: model,
		Input: input{
			Messages: []message{
				{
					Role: "user",
					Content: []contentItem{
						{Text: req.Prompt},
					},
				},
			},
		},
		Parameters: parameters,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal dashscope text2image request failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.BaseUrl, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("build dashscope text2image request failed: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call dashscope text2image failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read dashscope text2image response failed: %w", err)
	}

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("dashscope text2image request failed: status=%d body=%s", httpResp.StatusCode, string(respBody))
	}

	return parseResponse(respBody)
}

func buildParameters(req *schema.Request, callOpts *text2image.CallOptions) map[string]any {
	parameters := make(map[string]any)

	// 默认参数
	parameters[paramPromptExtend] = true
	parameters[paramWatermark] = false
	parameters[paramN] = 1

	// 从请求中设置 size
	if strings.TrimSpace(req.Size) != "" {
		parameters[paramSize] = schema.ConvSizeMul(req.Size)
	}

	// 从 callOpts 中解析额外参数
	if callOpts.Extra != nil {
		if v, ok := callOpts.Extra[extraKeyNegativePrompt].(string); ok && v != "" {
			parameters[paramNegativePrompt] = v
		}
		if v, ok := callOpts.Extra[extraKeyPromptExtend].(bool); ok {
			parameters[paramPromptExtend] = v
		}
		if v, ok := callOpts.Extra[extraKeyWatermark].(bool); ok {
			parameters[paramWatermark] = v
		}
		if v, ok := callOpts.Extra[extraKeyN].(int); ok && v > 0 {
			parameters[paramN] = v
		}
		if v, ok := callOpts.Extra[extraKeySeed].(int); ok {
			parameters[paramSeed] = v
		}
	}

	return parameters
}

// --- request payload ---

type apiRequest struct {
	Model      string         `json:"model"`
	Input      input          `json:"input"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

type input struct {
	Messages []message `json:"messages"`
}

type message struct {
	Role    string        `json:"role"`
	Content []contentItem `json:"content"`
}

type contentItem struct {
	Text  string `json:"text,omitempty"`  // for input
	Image string `json:"image,omitempty"` // for output
}

// --- response parsing ---

type apiResponse struct {
	Output    output   `json:"output"`
	Usage     apiUsage `json:"usage"`
	RequestID string   `json:"request_id"`
	Code      string   `json:"code,omitempty"`
	Message   string   `json:"message,omitempty"`
}

type output struct {
	Choices []choice `json:"choices"`
}

type choice struct {
	FinishReason string  `json:"finish_reason"`
	Message      message `json:"message"`
}

type apiUsage struct {
	Height     int `json:"height"`
	Width      int `json:"width"`
	ImageCount int `json:"image_count"`
}

func parseResponse(respBody []byte) (*schema.Response, error) {
	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode dashscope text2image response failed: %w", err)
	}

	// 检查错误响应
	if apiResp.Code != "" {
		return nil, fmt.Errorf("dashscope text2image error: code=%s message=%s", apiResp.Code, apiResp.Message)
	}

	if len(apiResp.Output.Choices) == 0 {
		return nil, fmt.Errorf("dashscope text2image response has no choices")
	}

	// 提取图片 URL
	choice := apiResp.Output.Choices[0]
	if len(choice.Message.Content) == 0 {
		return nil, fmt.Errorf("dashscope text2image response has no content")
	}

	// 构建 extras
	extras := make(map[string]any)
	if apiResp.RequestID != "" {
		extras["request_id"] = apiResp.RequestID
	}
	if apiResp.Usage.Height > 0 {
		extras["height"] = apiResp.Usage.Height
	}
	if apiResp.Usage.Width > 0 {
		extras["width"] = apiResp.Usage.Width
	}
	if apiResp.Usage.ImageCount > 0 {
		extras["image_count"] = apiResp.Usage.ImageCount
	}

	// 默认返回 URL 格式的第一张图片
	imageURL := choice.Message.Content[0].Image

	return &schema.Response{
		ResponseFormat: schema.ResponseFormatURL,
		ImageURL:       imageURL,
		Extras:         extras,
	}, nil
}
