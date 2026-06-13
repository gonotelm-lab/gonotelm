package prompts

import (
	"errors"
	"testing"
)

func TestNewChatTemplateManagerDefaultLang(t *testing.T) {
	manager, err := newChatTemplateManager("", func(lang string) (*ChatTemplate, error) {
		return &ChatTemplate{lang: lang}, nil
	})
	if err != nil {
		t.Fatalf("new chat template manager failed: %v", err)
	}

	defaultTemplate := manager.Get("")
	if defaultTemplate == nil {
		t.Fatalf("default template should not be nil")
	}
	if defaultTemplate.lang != defaultLang {
		t.Fatalf("unexpected default lang: %s", defaultTemplate.lang)
	}
}

func TestChatTemplateManagerCacheAndFallback(t *testing.T) {
	calls := map[string]int{}
	loader := func(lang string) (*ChatTemplate, error) {
		calls[lang]++
		switch lang {
		case "zh", "en":
			return &ChatTemplate{lang: lang}, nil
		default:
			return nil, errors.New("template not found")
		}
	}

	manager, err := newChatTemplateManager("zh", loader)
	if err != nil {
		t.Fatalf("new chat template manager failed: %v", err)
	}

	defaultTemplate := manager.Get("zh")
	if defaultTemplate == nil {
		t.Fatalf("default template should not be nil")
	}

	enTemplate1 := manager.Get("en")
	enTemplate2 := manager.Get(" en ")
	if enTemplate1 == nil || enTemplate2 == nil {
		t.Fatalf("english template should not be nil")
	}
	if enTemplate1 != enTemplate2 {
		t.Fatalf("english template should be cached")
	}
	if calls["en"] != 1 {
		t.Fatalf("english template should be loaded once, got %d", calls["en"])
	}

	fallbackTemplate1 := manager.Get("fr")
	fallbackTemplate2 := manager.Get("fr")
	if fallbackTemplate1 != defaultTemplate || fallbackTemplate2 != defaultTemplate {
		t.Fatalf("missing lang should fallback to default template")
	}
	if calls["fr"] != 2 {
		t.Fatalf("missing lang should retry loading each time, got %d", calls["fr"])
	}
}

func TestNewChatTemplateManagerWithoutLoader(t *testing.T) {
	_, err := newChatTemplateManager("zh", nil)
	if err == nil {
		t.Fatalf("expected error when loader is nil")
	}
}
