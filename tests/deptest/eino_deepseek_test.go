package deptest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/schema"
)

func TestEinoDeepSeekGenerate(t *testing.T) {
	apiKey := os.Getenv("GONOTELM_DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Fatal("GONOTELM_DEEPSEEK_API_KEY is empty")
	}

	ctx := context.Background()
	cm, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey:  apiKey,
		Model:   "deepseek-v4-flash",
		Timeout: 60 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	msgs := []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are a helpful assistant. Keep answers concise.",
		},
		{
			Role:    schema.User,
			Content: "用一句话解释什么是 LLM。",
		},
	}

	resp, err := cm.Generate(ctx, msgs)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("assistant: %s", resp.Content)
	if reasoning, ok := deepseek.GetReasoningContent(resp); ok {
		t.Logf("reasoning: %s", reasoning)
	}
	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		u := resp.ResponseMeta.Usage
		t.Logf("tokens: total=%d prompt=%d completion=%d",
			u.TotalTokens, u.PromptTokens, u.CompletionTokens)
	}
}
