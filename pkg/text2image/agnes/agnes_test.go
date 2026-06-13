package agnes

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gonotelm-lab/gonotelm/pkg/text2image/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/text2image/util"
)

func getAPIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("ENV_GONOTELM_AGNES_API_KEY")
	if key == "" {
		t.Skip("ENV_GONOTELM_AGNES_APIKEY not set, skipping integration test")
	}
	return key
}

func TestGenerate_Basic(t *testing.T) {
	gen, err := New(Config{APIKey: getAPIKey(t)})
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}

	resp, err := gen.Generate(t.Context(), &schema.Request{
		Prompt: "一只可爱的橘猫坐在窗台上，阳光照射进来，温暖舒适的氛围",
	})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	output, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Printf("Response: %s\n", string(output))
}

func TestGenerate_WithBase64Output(t *testing.T) {
	gen, err := New(Config{APIKey: getAPIKey(t)})
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}

	resp, err := gen.Generate(t.Context(), &schema.Request{
		Prompt:         "一只可爱的柯基狗狗在草地上玩耍，阳光照射进来，温暖舒适的氛围",
		ResponseFormat: schema.ResponseFormatBase64,
	})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	if resp.ImageBase64 == "" {
		t.Fatalf("expected non-empty image base64")
	}

	reader, err := util.ResolveResponse(resp)
	if err != nil {
		t.Fatalf("resolve response failed: %v", err)
	}
	b, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	if err := os.WriteFile("/tmp/image.png", b, 0644); err != nil {
		t.Fatalf("write image to file failed: %v", err)
	}
}

func TestGenerate_WithURLOutput(t *testing.T) {
	gen, err := New(Config{APIKey: getAPIKey(t)})
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}

	resp, err := gen.Generate(t.Context(), &schema.Request{
		Prompt: "a beautiful landscape with a river and a mountain",
	})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	reader, err := util.ResolveResponse(resp)
	if err != nil {
		t.Fatalf("resolve response failed: %v", err)
	}
	defer reader.Close()
	mimeType, err := mimetype.DetectReader(reader)
	if err != nil {
		t.Fatalf("detect mime type failed: %v", err)
	}
	fmt.Printf("mime type: %s\n", mimeType.String())

	b, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}

	if err := os.WriteFile("/tmp/image.png", b, 0644); err != nil {
		t.Fatalf("write image to file failed: %v", err)
	}

}
