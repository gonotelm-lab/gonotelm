package agnes

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func TestAgnesChatModel(t *testing.T) {
	apiKey := os.Getenv("ENV_GONOTELM_AGNES_API_KEY")
	if apiKey == "" {
		t.Skip("ENV_GONOTELM_AGNES_API_KEY not set, skipping test")
	}

	cfg := &ChatModelConfig{
		APIKey:  apiKey,
		Model:   "agnes-2.0-flash", // Default fallback model
		Timeout: 30 * time.Second,
	}

	model, err := NewChatModel(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create agnes model: %v", err)
	}

	msg := []*schema.Message{
		{
			Role:    schema.User,
			Content: "Hello! Reply with exactly 'Hi' and nothing else.",
		},
	}

	out, err := model.Generate(context.Background(), msg, WithExtraFields(map[string]any{
		"chat_template_kwargs": map[string]any{
			"enable_thinking": true,
		},
	}))
	if err != nil {
		t.Fatalf("failed to generate message: %v", err)
	}

	outJSON, err := json.MarshalIndent(out, "", " ")
	if err != nil {
		t.Fatalf("failed to marshal output: %v", err)
	}
	t.Logf("Response: %s", string(outJSON))
}

func TestAgnesChatModel_StreamAndThinking(t *testing.T) {
	apiKey := os.Getenv("ENV_GONOTELM_AGNES_API_KEY")
	if apiKey == "" {
		t.Skip("ENV_GONOTELM_AGNES_API_KEY not set, skipping test")
	}

	cfg := &ChatModelConfig{
		APIKey: apiKey,
		Model:  "agnes-2.0-flash",
	}

	model, err := NewChatModel(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create agnes model: %v", err)
	}

	msg := []*schema.Message{
		{
			Role:    schema.User,
			Content: "为什么天空是蓝色的？请一步步思考并解释。",
		},
	}

	stream, err := model.Stream(context.Background(), msg, WithExtraFields(
		map[string]any{
			"chat_template_kwargs": map[string]any{
				"enable_thinking": true,
			},
		},
	))
	if err != nil {
		t.Fatalf("failed to generate message stream: %v", err)
	}

	for {
		out, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}

		if out.ReasoningContent != "" {
			t.Logf("Thinking: %s", out.ReasoningContent)
		}
		if out.Content != "" {
			t.Logf("Content: %s", out.Content)
		}
	}
}
