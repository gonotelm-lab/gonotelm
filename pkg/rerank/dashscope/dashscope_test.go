package dashscope

import (
	"fmt"
	"os"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/rerank/schema"
)

func getAPIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("ENV_GONOTELM_OPENAI_API_KEY")
	if key == "" {
		t.Skip("ENV_GONOTELM_OPENAI_API_KEY not set, skipping integration test")
	}
	return key
}

func TestRerank_StringQuery(t *testing.T) {
	rr, err := New(Config{APIKey: getAPIKey(t)})
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}

	resp, err := rr.Rerank(t.Context(), schema.Request{
		Query: schema.NewStringQuery("什么是文本排序模型"),
		Documents: []schema.Document{
			{Parts: []schema.Part{{Type: schema.PartTypeText, Text: "文本排序模型是一种语义理解模型"}}},
			{Parts: []schema.Part{{Type: schema.PartTypeText, Text: "今天天气不错"}}},
			{Parts: []schema.Part{{Type: schema.PartTypeText, Text: "reranker用于对检索结果重排序"}}},
		},
	})
	if err != nil {
		t.Fatalf("rerank failed: %v", err)
	}

	if len(resp.Results) == 0 {
		t.Fatal("expected non-empty results")
	}
	if resp.Usage.TotalTokens <= 0 {
		t.Fatalf("expected positive total_tokens, got %d", resp.Usage.TotalTokens)
	}

	fmt.Printf("StringQuery results: %+v\n", resp.Results)
	fmt.Printf("Usage: %+v\n", resp.Usage)
}

func TestRerank_TextQueryObject(t *testing.T) {
	rr, err := New(Config{APIKey: getAPIKey(t)})
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}

	resp, err := rr.Rerank(t.Context(), schema.Request{
		Query: schema.NewTextQuery("什么是文本排序模型"),
		Documents: []schema.Document{
			{Parts: []schema.Part{{Type: schema.PartTypeText, Text: "文本排序模型是一种语义理解模型"}}},
			{Parts: []schema.Part{{Type: schema.PartTypeText, Text: "猫是一种可爱的动物"}}},
		},
		TopN: 1,
	})
	if err != nil {
		t.Fatalf("rerank failed: %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result with topN=1, got %d", len(resp.Results))
	}

	fmt.Printf("TextQueryObject results: %+v\n", resp.Results)
	fmt.Printf("Usage: %+v\n", resp.Usage)
}

func TestRerank_WithInstruct(t *testing.T) {
	rr, err := New(Config{APIKey: getAPIKey(t)})
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}

	resp, err := rr.Rerank(t.Context(), schema.Request{
		Query: schema.NewStringQuery("高性能网络框架"),
		Documents: []schema.Document{
			{Parts: []schema.Part{{Type: schema.PartTypeText, Text: "Netty是Java的异步事件驱动网络框架"}}},
			{Parts: []schema.Part{{Type: schema.PartTypeText, Text: "Django是Python的Web框架"}}},
			{Parts: []schema.Part{{Type: schema.PartTypeText, Text: "Go语言的net包提供高性能网络编程能力"}}},
		},
	}, WithInstruct("根据与查询的语义相关性对文档进行排序"))
	if err != nil {
		t.Fatalf("rerank failed: %v", err)
	}

	if len(resp.Results) == 0 {
		t.Fatal("expected non-empty results")
	}

	fmt.Printf("WithInstruct results: %+v\n", resp.Results)
}
