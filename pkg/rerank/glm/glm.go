package glm

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

// https://docs.bigmodel.cn/api-reference/%E6%A8%A1%E5%9E%8B-api/%E6%96%87%E6%9C%AC%E9%87%8D%E6%8E%92%E5%BA%8F

const (
	defaultBaseUrl = "https://open.bigmodel.cn/api/paas/v4/rerank"
	defaultModel   = "rerank"
	fieldText      = "text"
	fieldImage     = "image"
	fieldVideo     = "video"

	respFieldCreated   = "created"
	respFieldID        = "id"
	respFieldRequestID = "request_id"
	respFieldResults   = "results"
	respFieldUsage     = "usage"
)

type Reranker struct {
	cfg        Config
	httpClient *http.Client
}

func New(cfg Config, opts ...rerank.ClientOption) (*Reranker, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("glm api key is required")
	}
	if strings.TrimSpace(cfg.BaseUrl) == "" {
		cfg.BaseUrl = defaultBaseUrl
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

func (r *Reranker) Rerank(ctx context.Context, req *schema.Request, opts ...rerank.Option) (schema.Response, error) {
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

	payload := apiRequest{
		Model:           model,
		Query:           queryPayload,
		TopN:            topN,
		Documents:       documentsPayload,
		ReturnDocuments: req.ReturnDocuments,
	}

	callOpts := rerank.BuildCallOptions(opts...)
	if callOpts.Extra != nil {
		if returnDocuments, ok := callOpts.Extra[extraKeyReturnDocuments].(bool); ok {
			payload.ReturnDocuments = returnDocuments
		}
		if returnRawScores, ok := callOpts.Extra[extraKeyReturnRawScores].(bool); ok {
			payload.ReturnRawScores = returnRawScores
		}
		if requestID, ok := callOpts.Extra[extraKeyRequestID].(string); ok && strings.TrimSpace(requestID) != "" {
			payload.RequestID = requestID
		}
		if userID, ok := callOpts.Extra[extraKeyUserID].(string); ok && strings.TrimSpace(userID) != "" {
			payload.UserID = userID
		}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return schema.Response{}, fmt.Errorf("marshal glm rerank request failed: %w", err)
	}

	url := r.cfg.BaseUrl
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return schema.Response{}, fmt.Errorf("build glm rerank request failed: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return schema.Response{}, fmt.Errorf("call glm rerank failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return schema.Response{}, fmt.Errorf("read glm rerank response failed: %w", err)
	}

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
		return schema.Response{}, fmt.Errorf("glm rerank request failed: status=%d body=%s", httpResp.StatusCode, string(respBody))
	}

	return parseResponse(respBody)
}

type apiRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	TopN            int      `json:"top_n"`
	Documents       []string `json:"documents"`
	ReturnDocuments bool     `json:"return_documents,omitempty"`
	ReturnRawScores bool     `json:"return_raw_scores,omitempty"`
	RequestID       string   `json:"request_id,omitempty"`
	UserID          string   `json:"user_id,omitempty"`
}

func buildQuery(q schema.Query) (string, error) {
	if q.IsString() {
		query := strings.TrimSpace(q.String)
		if query == "" {
			return "", fmt.Errorf("query must not be empty")
		}
		return query, nil
	}

	if q.Object == nil || strings.TrimSpace(q.Object.Text) == "" {
		return "", fmt.Errorf("query object only supports text")
	}
	return strings.TrimSpace(q.Object.Text), nil
}

func buildDocuments(documents []schema.Document) ([]string, error) {
	if len(documents) == 0 {
		return nil, fmt.Errorf("documents must not be empty")
	}

	payload := make([]string, 0, len(documents))
	for docIdx, doc := range documents {
		text, err := buildDocumentText(docIdx, doc)
		if err != nil {
			return nil, err
		}
		payload = append(payload, text)
	}
	return payload, nil
}

func buildDocumentText(docIdx int, doc schema.Document) (string, error) {
	if doc.Part != nil {
		if doc.Part.Type != schema.PartTypeText {
			return "", fmt.Errorf(
				"document[%d] part type %q is not supported: only text part is supported",
				docIdx,
				doc.Part.Type,
			)
		}
		if strings.TrimSpace(doc.Part.Text) == "" {
			return "", fmt.Errorf("document[%d] part text must not be empty", docIdx)
		}
		return doc.Part.Text, nil
	}

	if strings.TrimSpace(doc.Text) == "" {
		return "", fmt.Errorf("document[%d] text and part must not both be empty", docIdx)
	}
	return doc.Text, nil
}

type apiResponse struct {
	Created   int64       `json:"created"`
	ID        string      `json:"id"`
	RequestID string      `json:"request_id"`
	Results   []apiResult `json:"results"`
	Usage     apiUsage    `json:"usage"`
}

type apiResult struct {
	Document       any     `json:"document"`
	Index          int     `json:"index"`
	RelevanceScore float32 `json:"relevance_score"`
}

type apiUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

func parseResponse(respBody []byte) (schema.Response, error) {
	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return schema.Response{}, fmt.Errorf("decode glm rerank response failed: %w", err)
	}

	results := make([]schema.Result, 0, len(apiResp.Results))
	for _, r := range apiResp.Results {
		item := schema.Result{
			Index:          r.Index,
			RelevanceScore: r.RelevanceScore,
		}
		if doc := parseResultDocument(r.Document); doc != nil {
			item.Document = doc
		}
		results = append(results, item)
	}

	resp := schema.Response{
		Results: results,
		Usage: schema.Usage{
			TotalTokens: apiResp.Usage.TotalTokens,
		},
	}

	extra := map[string]any{}
	if apiResp.ID != "" {
		extra[respFieldID] = apiResp.ID
	}
	if apiResp.RequestID != "" {
		extra[respFieldRequestID] = apiResp.RequestID
	}
	if apiResp.Created > 0 {
		extra[respFieldCreated] = apiResp.Created
	}
	if apiResp.Usage.PromptTokens > 0 {
		extra["prompt_tokens"] = apiResp.Usage.PromptTokens
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(respBody, &rawMap); err == nil {
		knownKeys := map[string]bool{
			respFieldCreated:   true,
			respFieldID:        true,
			respFieldRequestID: true,
			respFieldResults:   true,
			respFieldUsage:     true,
		}
		for k, v := range rawMap {
			if knownKeys[k] {
				continue
			}
			var val any
			_ = json.Unmarshal(v, &val)
			extra[k] = val
		}
	}

	if len(extra) > 0 {
		resp.Extra = extra
	}

	return resp, nil
}

func parseResultDocument(raw any) *schema.Document {
	switch doc := raw.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(doc) == "" {
			return nil
		}
		return &schema.Document{Text: doc}
	case map[string]any:
		result := &schema.Document{}
		text, _ := doc[fieldText].(string)
		if strings.TrimSpace(text) != "" {
			result.Text = text
		}
		image, _ := doc[fieldImage].(string)
		if strings.TrimSpace(image) != "" {
			result.Part = &schema.Part{
				Type: schema.PartTypeImage,
				Image: &schema.Image{
					URL: ptr(strings.TrimSpace(image)),
				},
			}
		}
		video, _ := doc[fieldVideo].(string)
		if strings.TrimSpace(video) != "" {
			result.Part = &schema.Part{
				Type: schema.PartTypeVideo,
				Video: &schema.Video{
					URL: strings.TrimSpace(video),
				},
			}
		}
		if result.Text != "" || result.Part != nil {
			return result
		}
	}
	return nil
}

func ptr[T any](v T) *T {
	return &v
}
