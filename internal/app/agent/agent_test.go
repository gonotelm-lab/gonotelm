package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	chat "github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var testLLM einomodel.ToolCallingChatModel

func TestMain(m *testing.M) {
	model, err := chat.New(context.Background(),
		chat.Openai,
		&chat.ProviderConfig{
			Openai: chat.OpenaiConfig{
				ApiKey:  os.Getenv("ENV_GONOTELM_OPENAI_API_KEY"),
				BaseUrl: os.Getenv("ENV_GONOTELM_OPENAI_BASE_URL"),
				Model:   os.Getenv("ENV_GONOTELM_OPENAI_MODEL"),
			},
		})
	if err != nil {
		panic(err)
	}

	testLLM = model

	m.Run()
}

func TestAgent_ReactStreamV2(t *testing.T) {
	agent := New(Config[struct{}]{
		LLM: testLLM,
		OnReasoningStart: func(ctx context.Context, round int, state struct{}) error {
			fmt.Println("reasoning start")
			return nil
		},
		OnReasoningDelta: func(ctx context.Context, round int, state struct{}, delta string) error {
			fmt.Println("reasoning delta", delta)
			return nil
		},
		OnReasoningEnd: func(ctx context.Context, round int, state struct{}) error {
			fmt.Println("reasoning end")
			return nil
		},
		OnContentStart: func(ctx context.Context, round int, state struct{}) error {
			fmt.Println("content start")
			return nil
		},
		OnContentDelta: func(ctx context.Context, round int, state struct{}, delta string) error {
			fmt.Println("content delta", delta)
			return nil
		},
		OnContentEnd: func(ctx context.Context, round int, state struct{}) error {
			fmt.Println("content end")
			return nil
		},
	}, struct{}{})

	final, err := agent.ReactStream(t.Context(), []*schema.Message{
		{
			Role:    schema.User,
			Content: "随便输出一点内容",
		},
	})
	if err != nil {
		fmt.Println("error", err)
		return
	}

	bb, _ := json.MarshalIndent(final, "", "  ")
	fmt.Println("final", string(bb))
}
