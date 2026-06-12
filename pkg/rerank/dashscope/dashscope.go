package dashscope

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gonotelm-lab/gonotelm/pkg/rerank"
	"github.com/gonotelm-lab/gonotelm/pkg/rerank/schema"
)

const (
	defaultBaseURL = "https://dashscope.aliyuncs.com"
	defaultPath    = "/compatible-api/v1/reranks"
	defaultModel   = "qwen3-rerank"
)

type Reranker struct {
	cfg        Config
	httpClient *http.Client
}

func New(cfg Config, opts ...rerank.ClientOption) (*Reranker, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("dashscope api key is required")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if strings.TrimSpace(cfg.Path) == "" {
		cfg.Path = defaultPath
	}
	if strings.TrimSpace(cfg.Model) == "" {
		cfg.Model = defaultModel
	}

	co := rerank.BuildClientOptions(cfg.Timeout, opts...)
	return &Reranker{
		cfg:        cfg,
		httpClient: co.HTTPClient,
	}, nil
}

func (r *Reranker) Rerank(ctx context.Context, req schema.Request, opts ...rerank.Option) (schema.Response, error) {
	topN, err := schema.NormalizeTopN(req.TopN, len(req.Documents))
	if err != nil {
		return schema.Response{}, err
	}

	queryPayload, err := buildQuery(req.Query)
	if err != nil {
		return schema.Response{}, err
	}
	documentsPayload, err := buildDocuments(req.Documents)
	if err != nil {
		return schema.Response{}, err
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = r.cfg.Model
	}

	callOpts := rerank.BuildCallOptions(opts...)
	parameters := map[string]any{
		paramTopN:            topN,
		paramReturnDocuments: false,
	}
	if callOpts.Extra != nil {
		if instruct, ok := callOpts.Extra[extraKeyInstruct].(string); ok && instruct != "" {
			parameters[paramInstruct] = instruct
		}
		if fps, ok := callOpts.Extra[extraKeyFPS].(float64); ok && fps > 0 {
			parameters[paramFPS] = fps
		}
	}

	payload := apiRequest{
		Model:      model,
		Query:      queryPayload,
		Documents:  documentsPayload,
		TopN:       topN,
		Parameters: parameters,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return schema.Response{}, fmt.Errorf("marshal dashscope rerank request failed: %w", err)
	}

	url := strings.TrimRight(r.cfg.BaseURL, "/") + r.cfg.Path
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return schema.Response{}, fmt.Errorf("build dashscope rerank request failed: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return schema.Response{}, fmt.Errorf("call dashscope rerank failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return schema.Response{}, fmt.Errorf("read dashscope rerank response failed: %w", err)
	}

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
		return schema.Response{}, fmt.Errorf("dashscope rerank request failed: status=%d body=%s", httpResp.StatusCode, string(respBody))
	}

	return parseResponse(respBody)
}

// --- request payload ---

type apiRequest struct {
	Model      string         `json:"model"`
	Query      any            `json:"query"`
	Documents  []any          `json:"documents"`
	TopN       int            `json:"top_n"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

func buildQuery(q schema.Query) (any, error) {
	if q.IsString() {
		if strings.TrimSpace(q.String) == "" {
			return nil, fmt.Errorf("query must not be empty")
		}
		return q.String, nil
	}

	obj := q.Object
	if obj == nil {
		return nil, fmt.Errorf("query object must not be nil")
	}

	result := map[string]string{}
	if t := strings.TrimSpace(obj.Text); t != "" {
		result[fieldText] = t
	}
	if img := strings.TrimSpace(obj.Image); img != "" {
		result[fieldImage] = img
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("query object must have at least text or image")
	}
	return result, nil
}

func buildDocuments(documents []schema.Document) ([]any, error) {
	if len(documents) == 0 {
		return nil, fmt.Errorf("documents must not be empty")
	}

	payload := make([]any, 0, len(documents))
	for idx, doc := range documents {
		if len(doc.Parts) == 0 {
			return nil, fmt.Errorf("document[%d] parts must not be empty", idx)
		}

		obj, err := buildPartObject(doc.Parts)
		if err != nil {
			return nil, fmt.Errorf("build document[%d] failed: %w", idx, err)
		}
		payload = append(payload, obj)
	}
	return payload, nil
}

func buildPartObject(parts []schema.Part) (map[string]any, error) {
	var (
		texts     []string
		imageData string
		videoData string
	)

	for idx, part := range parts {
		switch part.Type {
		case schema.PartTypeText:
			if strings.TrimSpace(part.Text) == "" {
				continue
			}
			texts = append(texts, part.Text)
		case schema.PartTypeImage:
			if imageData != "" {
				return nil, fmt.Errorf("part[%d] duplicated image content", idx)
			}
			img, err := buildImageValue(part.Image)
			if err != nil {
				return nil, fmt.Errorf("part[%d] image invalid: %w", idx, err)
			}
			imageData = img
		case schema.PartTypeVideo:
			if videoData != "" {
				return nil, fmt.Errorf("part[%d] duplicated video content", idx)
			}
			video, err := buildVideoValue(part.Video)
			if err != nil {
				return nil, fmt.Errorf("part[%d] video invalid: %w", idx, err)
			}
			videoData = video
		default:
			return nil, fmt.Errorf("part[%d] type %q is not supported", idx, part.Type)
		}
	}

	if len(texts) == 0 && imageData == "" && videoData == "" {
		return nil, fmt.Errorf("parts contain no valid content")
	}

	obj := map[string]any{}
	if len(texts) > 0 {
		obj[fieldText] = strings.Join(texts, "\n")
	}
	if imageData != "" {
		obj[fieldImage] = imageData
	}
	if videoData != "" {
		obj[fieldVideo] = videoData
	}
	return obj, nil
}

func buildImageValue(image *schema.Image) (string, error) {
	if image == nil {
		return "", fmt.Errorf("image must not be nil")
	}

	hasURL := image.URL != nil && strings.TrimSpace(*image.URL) != ""
	hasBase64 := image.Base64Data != nil && strings.TrimSpace(*image.Base64Data) != ""
	if hasURL == hasBase64 {
		return "", fmt.Errorf("image url and base64_data must be one-of")
	}

	if hasURL {
		return strings.TrimSpace(*image.URL), nil
	}

	base64 := strings.TrimSpace(*image.Base64Data)
	mimeType := strings.TrimSpace(image.MIMEType)
	if mimeType == "" {
		return base64, nil
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64), nil
}

func buildVideoValue(video *schema.Video) (string, error) {
	if video == nil {
		return "", fmt.Errorf("video must not be nil")
	}
	if strings.TrimSpace(video.URL) == "" {
		return "", fmt.Errorf("video url must not be empty")
	}
	return strings.TrimSpace(video.URL), nil
}

// --- response parsing (typed structs) ---

type apiResponse struct {
	ID      string      `json:"id"`
	Results []apiResult `json:"results"`
	Usage   apiUsage    `json:"usage"`
	Model   string      `json:"model"`
}

type apiResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

type apiUsage struct {
	TotalTokens int `json:"total_tokens"`
}

func parseResponse(respBody []byte) (schema.Response, error) {
	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return schema.Response{}, fmt.Errorf("decode dashscope rerank response failed: %w", err)
	}

	results := make([]schema.Result, 0, len(apiResp.Results))
	for _, r := range apiResp.Results {
		results = append(results, schema.Result{
			Index:          r.Index,
			RelevanceScore: r.RelevanceScore,
		})
	}

	resp := schema.Response{
		Results: results,
		Usage: schema.Usage{
			TotalTokens: apiResp.Usage.TotalTokens,
		},
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(respBody, &rawMap); err == nil {
		extra := map[string]any{}
		knownKeys := map[string]bool{respFieldResults: true, respFieldUsage: true, respFieldModel: true, respFieldID: true}
		for k, v := range rawMap {
			if knownKeys[k] {
				continue
			}
			var val any
			_ = json.Unmarshal(v, &val)
			extra[k] = val
		}
		if apiResp.ID != "" {
			extra[respFieldID] = apiResp.ID
		}
		if len(extra) > 0 {
			resp.Extra = extra
		}
	}

	return resp, nil
}
