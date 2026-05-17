package prompts

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestTemplateMessage(t *testing.T) {
	tmpl := newTemplate[ChatTemplateVars](templateNameChat, "zh")

	msg, err := tmpl.Message(context.Background(), ChatTemplateVars{
		Notebook: "项目笔记",
		SelectedSources: []ChatSelectedSourceGroup{
			{
				SourceID: "source:1",
				Docs: []ChatSelectedSourceDoc{
					{
						DocID:   "doc:1",
						Content: "文档片段",
						Score:   0.98,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("render message failed: %v", err)
	}

	if msg.Role != schema.System {
		t.Fatalf("unexpected role: %s", msg.Role)
	}

	if !strings.Contains(msg.Content, "项目笔记") {
		t.Fatalf("render result does not contain notebook variable")
	}

	if !strings.Contains(msg.Content, "文档片段") {
		t.Fatalf("render result does not contain selected source doc content")
	}

}

func TestTemplateMessageWithoutSelectedSources_NoCitationSpec(t *testing.T) {
	tmpl := newTemplate[ChatTemplateVars](templateNameChat, "zh")

	msg, err := tmpl.Message(context.Background(), ChatTemplateVars{
		Notebook: "项目笔记",
	})
	if err != nil {
		t.Fatalf("render message failed: %v", err)
	}

	if strings.Contains(msg.Content, "# 引用规范") {
		t.Fatalf("render result should not contain citation spec without selected sources")
	}
}

func TestTemplateDefaultLang(t *testing.T) {
	tmpl, err := NewChatTemplate("")
	if err != nil {
		t.Fatalf("new template with default lang failed: %v", err)
	}

	msg, err := tmpl.Message(context.Background(), ChatTemplateVars{})
	if err != nil {
		t.Fatalf("render default lang message failed: %v", err)
	}

	if msg.Role != schema.System {
		t.Fatalf("unexpected role: %s", msg.Role)
	}
}

func TestTemplateUnknownLang(t *testing.T) {
	tmpl := newTemplate[ChatTemplateVars](templateNameChat, "en")

	msg, err := tmpl.Message(context.Background(), ChatTemplateVars{})
	if err != nil {
		t.Fatalf("render fallback lang message failed: %v", err)
	}

	if msg.Role != schema.System {
		t.Fatalf("unexpected role: %s", msg.Role)
	}
}

func TestTemplateUnknownTemplateName(t *testing.T) {
	assertPanics(t, func() {
		_ = newTemplate[ChatTemplateVars](templateName("summary"), "zh")
	})
}

func TestTemplateEmptyTemplateName(t *testing.T) {
	assertPanics(t, func() {
		_ = newTemplate[ChatTemplateVars](templateName(""), "zh")
	})
}

func assertPanics(t *testing.T, fn func()) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic, but did not panic")
		}
	}()

	fn()
}
