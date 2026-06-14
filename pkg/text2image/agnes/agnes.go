package agnes

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

const (
	defaultBaseUrl = "https://apihub.agnes-ai.com/v1/images/generations"
	defaultModel   = "agnes-image-2.1-flash"
)

type Generator struct {
	cfg        Config
	httpClient *http.Client
}

func New(cfg Config, opts ...text2image.ClientOption) (*Generator, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("agnes api key is required")
	}
	if strings.TrimSpace(cfg.BaseUrl) == "" {
		cfg.BaseUrl = defaultBaseUrl
	} else {
		// Ensure it points to the correct endpoint if they just provided the base url
		if !strings.HasSuffix(cfg.BaseUrl, "/images/generations") {
			cfg.BaseUrl = strings.TrimRight(cfg.BaseUrl, "/") + "/images/generations"
		}
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

	// 模型优先级：Request > Config
	model := g.cfg.Model
	if req.Model != "" {
		model = req.Model
	}

	payload := apiRequest{
		Model:        model,
		Prompt:       req.Prompt,
		ReturnBase64: req.ResponseFormat == schema.ResponseFormatBase64,
	}
	if !payload.ReturnBase64 {
		payload.ExtraBody = &apiRequestExtraBody{
			ResponseFormat: "url",
		}
	}

	if strings.TrimSpace(req.Size) != "" {
		payload.Size = req.Size
		payload.Size = schema.ConvSizeX(req.Size)
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal agnes text2image request failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.BaseUrl, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("build agnes text2image request failed: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call agnes text2image failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read agnes text2image response failed: %w", err)
	}

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("agnes text2image request failed: status=%d body=%s", httpResp.StatusCode, string(respBody))
	}

	return parseResponse(respBody)
}

// --- request payload ---

type apiRequest struct {
	Model        string               `json:"model"`
	Prompt       string               `json:"prompt"`
	Size         string               `json:"size,omitempty"`
	ReturnBase64 bool                 `json:"return_base64,omitempty"`
	ExtraBody    *apiRequestExtraBody `json:"extra_body,omitempty"`
}

type apiRequestExtraBody struct {
	ResponseFormat string `json:"response_format,omitempty"`
}

// --- response parsing ---

type apiResponse struct {
	Created int64      `json:"created"`
	Data    []dataItem `json:"data"`
	Error   *apiError  `json:"error,omitempty"`
}

type dataItem struct {
	URL           string `json:"url"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

func parseResponse(respBody []byte) (*schema.Response, error) {
	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode agnes text2image response failed: %w", err)
	}

	// 检查错误响应
	if apiResp.Error != nil {
		return nil, fmt.Errorf("agnes text2image error: type=%s code=%s message=%s", apiResp.Error.Type, apiResp.Error.Code, apiResp.Error.Message)
	}

	if len(apiResp.Data) == 0 {
		return nil, fmt.Errorf("agnes text2image response has no data")
	}

	item := apiResp.Data[0]

	extras := make(map[string]any)
	if item.RevisedPrompt != "" {
		extras["revised_prompt"] = item.RevisedPrompt
	}

	// 优先返回 Base64 (如果存在)，否则返回 URL
	if item.B64JSON != "" {
		return &schema.Response{
			ResponseFormat: schema.ResponseFormatBase64,
			ImageBase64:    item.B64JSON,
			Extras:         extras,
		}, nil
	}

	if item.URL == "" {
		return nil, fmt.Errorf("agnes text2image response has no url or b64_json")
	}

	return &schema.Response{
		ResponseFormat: schema.ResponseFormatURL,
		ImageURL:       item.URL,
		Extras:         extras,
	}, nil
}
