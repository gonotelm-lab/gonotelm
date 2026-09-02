package adapter

import (
	"context"
	"encoding/base64"
	"net/url"
	"os"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
)

var (
	testGateway *chat.Gateway
)

func TestMain(m *testing.M) {
	var err error
	testGateway, err = chat.New(context.Background(), &chat.ProviderConfig{
		DeepSeek: chat.DeepSeekChatConfig{
			ApiKey:       os.Getenv("GONOTELM_OPENAI_API_KEY"),
			BaseURL:      "https://api.deepseek.com",
			DefaultModel: "deepseek-v4-flash",
			Models: map[string]chat.Model{
				"deepseek-v4-flash-vision-exp": {
					Name: "deepseek-v4-flash-vision-exp",
					Modalities: chat.Modality{
						Input:  []chat.ModalityType{chat.ModalityImage, chat.ModalityText},
						Output: []chat.ModalityType{chat.ModalityText},
					},
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	m.Run()
}

func TestImageInterpreter_Interpret(t *testing.T) {
	itpr, err := NewImageInterpreter(testGateway, chat.ProviderDeepSeek, "deepseek-v4-flash-vision-exp")
	if err != nil {
		t.Fatal(err)
	}

	result, err := itpr.Interpret(t.Context(), "https://cdn.deepseek.com/platform/favicon.png")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result)
}

func TestImageInterpreter_InterpretBase64(t *testing.T) {
	ctx := t.Context()
	itpr, err := NewImageInterpreter(testGateway, chat.ProviderDeepSeek, "deepseek-v4-flash-vision-exp")
	if err != nil {
		t.Fatal(err)
	}

	imageData, err := os.ReadFile("./testdata/image.png")
	if err != nil {
		t.Fatal(err)
	}
	base64Data := base64.StdEncoding.EncodeToString(imageData)

	result, err := itpr.Interpret(ctx, "data:image/png;base64,"+base64Data)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result)
}

func TestImageInterpreter_InterpretBytes(t *testing.T) {
	ctx := t.Context()
	itpr, err := NewImageInterpreter(testGateway, chat.ProviderDeepSeek, "deepseek-v4-flash-vision-exp")
	if err != nil {
		t.Fatal(err)
	}

	imageData, err := os.ReadFile("./testdata/image.png")
	if err != nil {
		t.Fatal(err)
	}

	result, err := itpr.InterpretBytes(ctx, imageData)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result)
}	

func TestImageDataUrlParse(t *testing.T) {
	u, err := url.Parse("data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADIA...")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(u) // it works
	t.Log(u.Scheme)
}