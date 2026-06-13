package dashscope

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/text2image/schema"
)

func getAPIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("ENV_GONOTELM_DASHSCOPE_APIKEY")
	if key == "" {
		t.Skip("ENV_GONOTELM_DASHSCOPE_APIKEY not set, skipping integration test")
	}
	return key
}

func TestGenerate_Basic(t *testing.T) {
	gen, err := New(Config{APIKey: getAPIKey(t)})
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}

	resp, err := gen.Generate(t.Context(), &schema.Request{
		Model:  "qwen-image-2.0",
		Prompt: "一只可爱的橘猫坐在窗台上，阳光照射进来，温暖舒适的氛围",
	})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	output, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Printf("Response: %s\n", string(output))
}

func TestGenerate_WithSize(t *testing.T) {
	gen, err := New(Config{APIKey: getAPIKey(t)})
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}

	resp, err := gen.Generate(t.Context(), &schema.Request{
		Prompt: "一座现代化的城市夜景，高楼大厦灯火通明",
		Size:   "1024*1024",
	})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if resp.ImageURL == "" {
		t.Fatal("expected non-empty image url")
	}

	// 检查 extras 中是否包含尺寸信息
	if _, ok := resp.Extras["width"]; !ok {
		t.Log("warning: expected width in extras")
	}
	if _, ok := resp.Extras["height"]; !ok {
		t.Log("warning: expected height in extras")
	}

	fmt.Printf("Image URL: %s\n", resp.ImageURL)
	fmt.Printf("Extras: %+v\n", resp.Extras)
}

func TestGenerate_WithOptions(t *testing.T) {
	gen, err := New(Config{APIKey: getAPIKey(t)})
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}

	resp, err := gen.Generate(t.Context(), &schema.Request{
		Prompt: "一片宁静的湖泊，周围是雪山和森林，倒影清晰可见",
		Size:   "1024*1024",
	},
		WithNegativePrompt("模糊, 低质量, 变形"),
		WithPromptExtend(false),
		WithWatermark(false),
		WithSeed(42),
	)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if resp.ImageURL == "" {
		t.Fatal("expected non-empty image url")
	}

	fmt.Printf("Image URL: %s\n", resp.ImageURL)
	fmt.Printf("Extras: %+v\n", resp.Extras)
}

func TestGenerate_InvalidAPIKey(t *testing.T) {
	gen, err := New(Config{APIKey: "invalid-key"})
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}

	_, err = gen.Generate(t.Context(), &schema.Request{
		Prompt: "test prompt",
	})
	if err == nil {
		t.Fatal("expected error with invalid api key")
	}

	fmt.Printf("Expected error: %v\n", err)
}

func TestGenerate_EmptyPrompt(t *testing.T) {
	gen, err := New(Config{APIKey: getAPIKey(t)})
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}

	_, err = gen.Generate(t.Context(), &schema.Request{
		Prompt: "",
	})
	if err == nil {
		t.Fatal("expected error with empty prompt")
	}

	fmt.Printf("Expected error: %v\n", err)
}

func TestNew_MissingAPIKey(t *testing.T) {
	_, err := New(Config{APIKey: ""})
	if err == nil {
		t.Fatal("expected error with empty api key")
	}

	fmt.Printf("Expected error: %v\n", err)
}
