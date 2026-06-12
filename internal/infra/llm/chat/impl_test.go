package chat

import (
	"fmt"
	"os"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestNew(t *testing.T) {
	model, err := New(t.Context(), Qwen, &ProviderConfig{
		Qwen: QwenConfig{
			ApiKey:  os.Getenv("ENV_GONOTELM_OPENAI_API_KEY"),
			BaseUrl: os.Getenv("ENV_GONOTELM_OPENAI_BASE_URL"),
			Model:   os.Getenv("ENV_GONOTELM_OPENAI_MODEL"),
		},
	})
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	result, err := model.Generate(t.Context(), []*schema.Message{
		{
			Role:    schema.User,
			Content: "Hello, how are you? Please output a valid json format response with field `name`, `age`, `greetings`.",
		},
	}, WithResponseJsonObject(Qwen))
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}

	fmt.Println(result.Content)
}
