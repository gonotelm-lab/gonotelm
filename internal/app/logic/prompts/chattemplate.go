package prompts

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

type ChatTemplateVars struct {
	Notebook        string
	SelectedSources []ChatSelectedSourceGroup
}

type ChatSelectedSourceGroup struct {
	SourceIndex int64
	SourceID    string
	Docs        []ChatSelectedSourceDoc
}

type ChatSelectedSourceDoc struct {
	DocIndex int64
	DocID    string
	Content  string
	Score    float32
}

func (v ChatTemplateVars) PromptVars() map[string]any {
	return map[string]any{
		"Notebook":        v.Notebook,
		"SelectedSources": v.SelectedSources,
	}
}

type ChatTemplate = template[ChatTemplateVars]

func NewChatTemplate(lang string) (*ChatTemplate, error) {
	return newTemplate[ChatTemplateVars](templateNameChat, lang), nil
}

func normalizeTemplateLang(lang string) string {
	normalizedLang := strings.TrimSpace(lang)
	if normalizedLang == "" {
		normalizedLang = defaultLang
	}

	return normalizedLang
}

// ChatTemplateManager manages chat templates cache and lazy loading.
type ChatTemplateManager struct {
	mu sync.RWMutex

	defaultLang string
	templates   map[string]*ChatTemplate
	loader      func(lang string) (*ChatTemplate, error)
}

func NewChatTemplateManager(defaultLanguage string) (*ChatTemplateManager, error) {
	return newChatTemplateManager(defaultLanguage, NewChatTemplate)
}

func newChatTemplateManager(
	defaultLanguage string,
	loader func(lang string) (*ChatTemplate, error),
) (*ChatTemplateManager, error) {
	normalizedLang := normalizeTemplateLang(defaultLanguage)
	if loader == nil {
		return nil, fmt.Errorf("chat template loader is required")
	}

	defaultTemplate, err := loader(normalizedLang)
	if err != nil {
		return nil, fmt.Errorf("init default chat template failed: %w", err)
	}

	return &ChatTemplateManager{
		defaultLang: normalizedLang,
		templates: map[string]*ChatTemplate{
			normalizedLang: defaultTemplate,
		},
		loader: loader,
	}, nil
}

func (m *ChatTemplateManager) Get(lang string) *ChatTemplate {
	normalizedLang := strings.TrimSpace(lang)
	if normalizedLang == "" {
		normalizedLang = m.defaultLang
	}

	m.mu.RLock()
	if tmpl, ok := m.templates[normalizedLang]; ok {
		m.mu.RUnlock()
		return tmpl
	}
	defaultTemplate := m.templates[m.defaultLang]
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if tmpl, ok := m.templates[normalizedLang]; ok {
		return tmpl
	}

	tmpl, err := m.loader(normalizedLang)
	if err != nil {
		slog.Warn("load chat prompt template failed, fallback to default",
			slog.String("lang", normalizedLang),
			slog.Any("err", err),
		)
		return defaultTemplate
	}

	m.templates[normalizedLang] = tmpl
	return tmpl
}
