package prompts

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestNotebookSummaryTemplateVarsPromptVars(t *testing.T) {
	promptVars := NotebookSummaryTemplateVars{
		Sources: []string{"  来源摘要A  ", " ", "\n来源摘要B\n"},
	}.PromptVars()

	sources, ok := promptVars["sources"].([]string)
	if !ok {
		t.Fatalf("sources should be []string")
	}
	if len(sources) != 2 {
		t.Fatalf("unexpected sources count: %d", len(sources))
	}
	if sources[0] != "来源摘要A" || sources[1] != "来源摘要B" {
		t.Fatalf("unexpected sources: %v", sources)
	}
}

func TestNotebookSummaryTemplateMessage(t *testing.T) {
	tmpl := NewNotebookSummaryTemplate("zh")
	msg, err := tmpl.Message(context.Background(), NotebookSummaryTemplateVars{
		Sources: []string{"来源摘要A", "来源摘要B"},
	})
	if err != nil {
		t.Fatalf("render notebook summary message failed: %v", err)
	}

	if msg.Role != schema.System {
		t.Fatalf("unexpected role: %s", msg.Role)
	}

	if !strings.Contains(msg.Content, "来源摘要A") {
		t.Fatalf("render result does not contain sources variable")
	}
	if !strings.Contains(msg.Content, "来源摘要B") {
		t.Fatalf("render result does not contain sources variable")
	}
}
