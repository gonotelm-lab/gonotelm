package prompt

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	chatmodel "github.com/gonotelm-lab/gonotelm/internal/app/model/chat"
)

func TestTemplateMessage(t *testing.T) {
	p := New("zh")
	tmpl := p.ChatTemplate("zh")

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

	if msg.Role != schema.User {
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
	p := New("zh")
	tmpl := p.ChatTemplate("zh")

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

func TestTemplateMessageStyleVariants(t *testing.T) {
	p := New("zh")
	tmpl := p.ChatTemplate("zh")

	cases := []struct {
		name           string
		style          chatmodel.ChatStyle
		expectContains string
	}{
		{
			name:           "default",
			style:          ChatTemplateStyleDefault,
			expectContains: "高标准的认知学习助手",
		},
		{
			name:           "analyst",
			style:          ChatTemplateStyleAnalyst,
			expectContains: "严谨的分析师型学习助手",
		},
		{
			name:           "guide",
			style:          ChatTemplateStyleGuide,
			expectContains: "教学向导型学习助手",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := tmpl.Message(context.Background(), ChatTemplateVars{
				Style: tt.style,
			})
			if err != nil {
				t.Fatalf("render message failed: %v", err)
			}

			if !strings.Contains(msg.Content, tt.expectContains) {
				t.Fatalf("render result does not contain expected style text: %s", tt.expectContains)
			}
		})
	}
}

func TestTemplateDefaultLang(t *testing.T) {
	p := New("zh")
	tmpl := p.ChatTemplate("zh")

	msg, err := tmpl.Message(context.Background(), ChatTemplateVars{})
	if err != nil {
		t.Fatalf("render default lang message failed: %v", err)
	}

	if msg.Role != schema.User {
		t.Fatalf("unexpected role: %s", msg.Role)
	}
}

func TestTemplateUnknownLang(t *testing.T) {
	p := New("zh")
	tmpl := p.ChatTemplate("en")

	msg, err := tmpl.Message(context.Background(), ChatTemplateVars{})
	if err != nil {
		t.Fatalf("render fallback lang message failed: %v", err)
	}

	if msg.Role != schema.User {
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
