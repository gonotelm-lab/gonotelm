package prompts

import (
	"context"
	"strings"
	"testing"
)

func TestRenderStudioMindmapV2Message(t *testing.T) {
	msg, err := RenderStudioMindmapV2Message(
		context.Background(),
		[]string{"src-1", "src-2"},
		"zh",
	)
	if err != nil {
		t.Fatalf("render studio mindmap v2 message failed: %v", err)
	}

	if !strings.Contains(msg.Content, "src-1") || !strings.Contains(msg.Content, "src-2") {
		t.Fatalf("render result does not contain source ids")
	}

	if strings.Contains(msg.Content, "{{Mode}}") {
		t.Fatalf("render result should not contain mode placeholder")
	}
}
