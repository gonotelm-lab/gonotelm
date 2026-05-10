package prompts

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestTemplateMessage(t *testing.T) {
	tmpl, err := NewTemplate[ChatTemplateVars](TemplateNameChat, "zh")
	if err != nil {
		t.Fatalf("new template failed: %v", err)
	}

	msg, err := tmpl.Message(context.Background(), ChatTemplateVars{
		Notebook: "项目笔记",
		SelectedSourceDocs: []ChatSelectedSourceDoc{
			{
				DocID:   "doc:1",
				Content: "文档片段",
				Score:   0.98,
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
	_, err := NewTemplate[ChatTemplateVars](TemplateNameChat, "en")
	if err == nil {
		t.Fatalf("expect error when lang does not exist")
	}
}

func TestTemplateUnknownTemplateName(t *testing.T) {
	_, err := NewTemplate[ChatTemplateVars]("summary", "zh")
	if err == nil {
		t.Fatalf("expect error when template name does not exist")
	}
}

func TestTemplateEmptyTemplateName(t *testing.T) {
	_, err := NewTemplate[ChatTemplateVars]("", "zh")
	if err == nil {
		t.Fatalf("expect error when template name is empty")
	}
}
