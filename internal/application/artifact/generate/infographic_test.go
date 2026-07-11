package generate

import (
	"context"
	"testing"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func TestInfoGraphicGenerator_ImplementsGenerator(t *testing.T) {
	var _ Generator = &InfoGraphicGenerator{}
}

func TestNewGenerator_InfoGraphicKind(t *testing.T) {
	g, err := newGenerator(artifactentity.KindInfoGraphic, &ServiceDeps{})
	if err != nil {
		t.Fatalf("expected success for KindInfoGraphic, got err=%v", err)
	}
	if _, ok := g.(*InfoGraphicGenerator); !ok {
		t.Fatalf("expected *InfoGraphicGenerator, got %T", g)
	}
}

func TestInfoGraphicGenerator_Generate_InvalidPayload(t *testing.T) {
	ig := &InfoGraphicGenerator{deps: &ServiceDeps{}}
	req := &Request{
		Kind:    artifactentity.KindInfoGraphic,
		Payload: &artifactentity.MindmapPayload{},
	}
	_, err := ig.Generate(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid payload type")
	}
}

func TestParseAgentOutput_Valid(t *testing.T) {
	ig := &InfoGraphicGenerator{}
	ctx := context.Background()

	expect, err := ig.parseAgentOutput(ctx, `{"title":"人工智能发展报告","image_prompt":"A futuristic infographic showing AI evolution"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expect.Title != "人工智能发展报告" {
		t.Errorf("expected title '人工智能发展报告', got %q", expect.Title)
	}
	if expect.ImagePrompt != "A futuristic infographic showing AI evolution" {
		t.Errorf("unexpected image_prompt: %q", expect.ImagePrompt)
	}
}

func TestParseAgentOutput_Empty(t *testing.T) {
	ig := &InfoGraphicGenerator{}
	ctx := context.Background()

	_, err := ig.parseAgentOutput(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty output")
	}
}

func TestParseAgentOutput_MissingImagePrompt(t *testing.T) {
	ig := &InfoGraphicGenerator{}
	ctx := context.Background()

	_, err := ig.parseAgentOutput(ctx, `{"title":"test"}`)
	if err == nil {
		t.Fatal("expected error for missing image_prompt")
	}
}

func TestParseAgentOutput_InvalidJSON(t *testing.T) {
	ig := &InfoGraphicGenerator{}
	ctx := context.Background()

	_, err := ig.parseAgentOutput(ctx, `not json at all`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseAgentOutput_ExtraSpaces(t *testing.T) {
	ig := &InfoGraphicGenerator{}
	ctx := context.Background()

	expect, err := ig.parseAgentOutput(ctx, `  {"title":"  AI Trend  ","image_prompt":"  prompt here  "}  `)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expect.Title != "AI Trend" {
		t.Errorf("expected title 'AI Trend', got %q", expect.Title)
	}
	if expect.ImagePrompt != "prompt here" {
		t.Errorf("unexpected image_prompt: %q", expect.ImagePrompt)
	}
}

func TestParseAgentOutput_TitleTruncation(t *testing.T) {
	ig := &InfoGraphicGenerator{}
	ctx := context.Background()

	longTitle := ""
	for i := 0; i < 200; i++ {
		longTitle += "测"
	}
	// After min title length check (10 runes), title is truncated to 30
	// Actually wait: in mindmap_test the condition is `titleLen > mindmapTitleMinLen` before truncation
	// But in infographic, title is ALWAYS truncated regardless
	// Let me re-check...
	// In parseAgentOutput: expect.Title = pkgstring.TruncateRune(expect.Title, constants.MaxArtifactTitleLength)
	// constants.MaxArtifactTitleLength = 128
	// So title > 128 runes will be truncated to 128
	// But for this test, a 200-rune title should be truncated to 128

	expect, err := ig.parseAgentOutput(ctx, `{"title":"`+longTitle+`","image_prompt":"prompt"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should be truncated to MaxArtifactTitleLength (128)
	if len([]rune(expect.Title)) > 128 {
		t.Errorf("expected title to be truncated to <=128 runes, got %d runes", len([]rune(expect.Title)))
	}
}

func TestFormatArtifactStoreKey_WithDotPrefix(t *testing.T) {
	nbId := uuid.MustParseString("00000000-0000-0000-0000-000000000001")
	taskId := uuid.MustParseString("00000000-0000-0000-0000-000000000002")
	key := formatArtifactStoreKey(nbId, taskId, ".png")
	expected := "artifact/" + nbId.String() + "/" + taskId.String() + ".png"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestFormatArtifactStoreKey_WithoutDotPrefix(t *testing.T) {
	nbId := uuid.MustParseString("00000000-0000-0000-0000-000000000001")
	taskId := uuid.MustParseString("00000000-0000-0000-0000-000000000002")
	key := formatArtifactStoreKey(nbId, taskId, "png")
	expected := "artifact/" + nbId.String() + "/" + taskId.String() + ".png"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestFormatArtifactStoreKey_Jpeg(t *testing.T) {
	nbId := uuid.MustParseString("00000000-0000-0000-0000-000000000003")
	taskId := uuid.MustParseString("00000000-0000-0000-0000-000000000004")
	key := formatArtifactStoreKey(nbId, taskId, ".jpeg")
	expected := "artifact/" + nbId.String() + "/" + taskId.String() + ".jpeg"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}
