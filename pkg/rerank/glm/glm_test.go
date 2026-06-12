package glm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pkgrerank "github.com/gonotelm-lab/gonotelm/pkg/rerank"
	"github.com/gonotelm-lab/gonotelm/pkg/rerank/schema"
)

func TestNew(t *testing.T) {
	t.Run("missing api key", func(t *testing.T) {
		_, err := New(Config{})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "glm api key is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("apply defaults", func(t *testing.T) {
		rr, err := New(Config{APIKey: "test-key"})
		if err != nil {
			t.Fatalf("new failed: %v", err)
		}

		if rr.cfg.BaseURL != defaultBaseURL {
			t.Fatalf("unexpected default base url: %q", rr.cfg.BaseURL)
		}
		if rr.cfg.Path != defaultPath {
			t.Fatalf("unexpected default path: %q", rr.cfg.Path)
		}
		if rr.cfg.Model != defaultModel {
			t.Fatalf("unexpected default model: %q", rr.cfg.Model)
		}
		if rr.httpClient == nil {
			t.Fatal("http client must not be nil")
		}
	})
}

func TestRerank_RequestAndResponse(t *testing.T) {
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/paas/v4/rerank" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected content type: %q", got)
		}

		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body failed: %v", err)
		}
		if err := json.Unmarshal(rawBody, &capturedBody); err != nil {
			t.Fatalf("decode request body failed: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"task-1",
			"request_id":"req-1",
			"created":1732083164,
			"results":[{"index":1,"relevance_score":0.99866986,"document":"文档B"}],
			"usage":{"prompt_tokens":72,"total_tokens":72}
		}`))
	}))
	defer srv.Close()

	rr, err := New(
		Config{
			APIKey:  "test-key",
			BaseURL: srv.URL,
			Path:    "/paas/v4/rerank",
			Model:   "rerank",
		},
		pkgrerank.WithHTTPClient(srv.Client()),
	)
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}

	resp, err := rr.Rerank(t.Context(), &schema.Request{
		Query: schema.NewTextQuery("什么是文本重排序"),
		Documents: []schema.Document{
			{
				Text: "文档A-text",
				Part: &schema.Part{
					Type: schema.PartTypeText,
					Text: "文档A-part",
				},
			},
			{Text: "文档B"},
		},
		TopN:            0,
		ReturnDocuments: true,
	}, WithRequestID("req-1"), WithUserID("user-1"), WithReturnRawScores(true))
	if err != nil {
		t.Fatalf("rerank failed: %v", err)
	}

	if capturedBody["model"] != "rerank" {
		t.Fatalf("unexpected model: %v", capturedBody["model"])
	}
	if capturedBody["query"] != "什么是文本重排序" {
		t.Fatalf("unexpected query payload: %v", capturedBody["query"])
	}
	if got := int(capturedBody["top_n"].(float64)); got != 2 {
		t.Fatalf("unexpected top_n: %d", got)
	}
	if got := capturedBody["return_raw_scores"]; got != true {
		t.Fatalf("unexpected return_raw_scores: %v", got)
	}
	if got := capturedBody["request_id"]; got != "req-1" {
		t.Fatalf("unexpected request_id: %v", got)
	}
	if got := capturedBody["user_id"]; got != "user-1" {
		t.Fatalf("unexpected user_id: %v", got)
	}
	if got := capturedBody["return_documents"]; got != true {
		t.Fatalf("unexpected return_documents: %v", got)
	}

	documents, ok := capturedBody["documents"].([]any)
	if !ok {
		t.Fatalf("documents type mismatch: %T", capturedBody["documents"])
	}
	if len(documents) != 2 || documents[0] != "文档A-part" || documents[1] != "文档B" {
		t.Fatalf("unexpected documents payload: %#v", documents)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("unexpected result size: %d", len(resp.Results))
	}
	if resp.Results[0].Index != 1 {
		t.Fatalf("unexpected index: %d", resp.Results[0].Index)
	}
	if resp.Results[0].RelevanceScore <= 0 {
		t.Fatalf("unexpected relevance score: %f", resp.Results[0].RelevanceScore)
	}
	if resp.Results[0].Document == nil || resp.Results[0].Document.Text != "文档B" {
		t.Fatalf("unexpected result document: %+v", resp.Results[0].Document)
	}
	if resp.Usage.TotalTokens != 72 {
		t.Fatalf("unexpected total tokens: %d", resp.Usage.TotalTokens)
	}

	if resp.Extra == nil {
		t.Fatal("extra should not be nil")
	}
	if got := resp.Extra["id"]; got != "task-1" {
		t.Fatalf("unexpected extra id: %v", got)
	}
	if got := resp.Extra["request_id"]; got != "req-1" {
		t.Fatalf("unexpected extra request_id: %v", got)
	}
}

func TestRerank_InvalidInput(t *testing.T) {
	rr, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}

	t.Run("image query is not supported", func(t *testing.T) {
		_, rerankErr := rr.Rerank(t.Context(), &schema.Request{
			Query: schema.NewImageQuery("https://example.com/a.png"),
			Documents: []schema.Document{
				{Text: "doc"},
			},
		})
		if rerankErr == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(rerankErr.Error(), "query object only supports text") {
			t.Fatalf("unexpected error: %v", rerankErr)
		}
	})

	t.Run("non-text document part is not supported", func(t *testing.T) {
		_, rerankErr := rr.Rerank(t.Context(), &schema.Request{
			Query: schema.NewStringQuery("query"),
			Documents: []schema.Document{
				{
					Part: &schema.Part{
						Type:  schema.PartTypeImage,
						Image: &schema.Image{URL: ptr("https://example.com/a.png")},
					},
				},
			},
		})
		if rerankErr == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(rerankErr.Error(), "only text part is supported") {
			t.Fatalf("unexpected error: %v", rerankErr)
		}
	})

	t.Run("negative top_n", func(t *testing.T) {
		_, rerankErr := rr.Rerank(t.Context(), &schema.Request{
			Query: schema.NewStringQuery("query"),
			Documents: []schema.Document{
				{Text: "doc"},
			},
			TopN: -1,
		})
		if rerankErr == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(rerankErr.Error(), "top_n must not be negative") {
			t.Fatalf("unexpected error: %v", rerankErr)
		}
	})
}
