package convertdoc

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func TestTextHandlerHandleSplitEffective(t *testing.T) {
	handler := NewTextHandler(HandlerConfig{
		ChunkSize:   120,
		OverlapSize: 20,
	})

	inputText := buildMixedLanguageText(90)
	source := mustBuildTextSource(t, inputText)

	result, err := handler.Handle(context.Background(), source)
	if err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	if len(result.Docs) <= 1 {
		t.Fatalf("expect split docs > 1, got %d", len(result.Docs))
	}
}

func mustBuildTextSource(t *testing.T, text string) *model.Source {
	t.Helper()

	content, err := sonic.Marshal(model.TextSourceContent{Text: text})
	if err != nil {
		t.Fatalf("marshal source content failed: %v", err)
	}

	return &model.Source{
		Id:         uuid.NewV4(),
		NotebookId: uuid.NewV4(),
		Kind:       model.SourceKindText,
		Content:    content,
	}
}

func buildMixedLanguageText(paragraphCount int) string {
	if paragraphCount <= 0 {
		return ""
	}

	var builder strings.Builder
	builder.Grow(paragraphCount * 140)

	for i := 1; i <= paragraphCount; i++ {
		builder.WriteString("第")
		builder.WriteString(strconv.Itoa(i))
		builder.WriteString("段：Rust 所有权与借用让并发更安全。")
		builder.WriteString("English note ")
		builder.WriteString(strconv.Itoa(i))
		builder.WriteString(": ownership, borrowing, and lifetimes improve reliability! ")
		builder.WriteString("再补一句中文，验证中英文混排和标点切分是否稳定。")
		builder.WriteString("Final sentence ")
		builder.WriteString(strconv.Itoa(i))
		builder.WriteString(".\n")
	}

	return builder.String()
}
