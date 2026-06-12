package schema

import (
	"encoding/json"
	"fmt"
)

type PartType string

const (
	PartTypeText  PartType = "text"
	PartTypeImage PartType = "image"
	PartTypeVideo PartType = "video"
)

type Image struct {
	// URL 和 Base64Data 二选一
	URL        *string `json:"url,omitempty"`
	Base64Data *string `json:"base64_data,omitempty"`
	MIMEType   string  `json:"mime_type,omitempty"`

	Extra map[string]any `json:"extra,omitempty"`
}

type Video struct {
	// 当前仅支持 URL。
	URL string `json:"url,omitempty"`

	MIMEType string         `json:"mime_type,omitempty"`
	Extra    map[string]any `json:"extra,omitempty"`
}

type Part struct {
	Type PartType `json:"type"`

	Text  string `json:"text,omitempty"`
	Image *Image `json:"image,omitempty"`
	Video *Video `json:"video,omitempty"`

	Extra map[string]any `json:"extra,omitempty"`
}

type Document struct {
	Parts []Part         `json:"parts"`
	Extra map[string]any `json:"extra,omitempty"`
}

// Query 表示 query string | object 的并集类型：
//  1. "query": "..."
//  2. "query": {"text":"..."} / {"image":"..."}
//
// 约定：若 Object 非 nil，则以 Object 为准；否则使用 String。
type Query struct {
	String string       `json:"-"`
	Object *QueryObject `json:"-"`
}

type QueryObject struct {
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
}

func NewStringQuery(s string) Query {
	return Query{String: s}
}

func NewTextQuery(text string) Query {
	return Query{Object: &QueryObject{Text: text}}
}

func NewImageQuery(image string) Query {
	return Query{Object: &QueryObject{Image: image}}
}

func (q Query) MarshalJSON() ([]byte, error) {
	if q.Object != nil {
		return json.Marshal(q.Object)
	}
	return json.Marshal(q.String)
}

func (q *Query) UnmarshalJSON(data []byte) error {
	if q == nil {
		return fmt.Errorf("query is nil")
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		q.String = s
		q.Object = nil
		return nil
	}

	var obj QueryObject
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("query must be string or object: %w", err)
	}

	q.String = ""
	q.Object = &obj
	return nil
}

func (q Query) IsObject() bool {
	return q.Object != nil
}

func (q Query) IsString() bool {
	return q.Object == nil
}

type Request struct {
	Query Query `json:"query"`

	Documents []Document `json:"documents"`
	TopN      int        `json:"top_n,omitempty"`
	Model     string     `json:"model,omitempty"`

	Extra map[string]any `json:"extra,omitempty"`
}

type Response struct {
	Results []Result `json:"results,omitempty"`
	Usage   Usage    `json:"usage,omitempty"`
	Extra   map[string]any `json:"extra,omitempty"`
}

type Result struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
	Extra          map[string]any `json:"extra,omitempty"`
}

type Usage struct {
	TotalTokens int `json:"total_tokens,omitempty"`
}
