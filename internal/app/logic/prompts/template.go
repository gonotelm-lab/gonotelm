package prompts

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

const (
	defaultLang      = "zh"
	TemplateNameChat = "chat"
)

//go:embed */*.jinja
var templateFiles embed.FS

type TemplateVars interface {
	PromptVars() map[string]any
}

type Template[T TemplateVars] struct {
	name string
	lang string
	tmpl prompt.ChatTemplate
}

func NewTemplate[T TemplateVars](name, lang string) (*Template[T], error) {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return nil, errors.New("template name is required")
	}

	normalizedLang := strings.TrimSpace(lang)
	if normalizedLang == "" {
		normalizedLang = defaultLang
	}

	content, err := readTemplate(normalizedName, normalizedLang)
	if err != nil {
		return nil, err
	}

	return &Template[T]{
		name: normalizedName,
		lang: normalizedLang,
		tmpl: prompt.FromMessages(schema.Jinja2, schema.SystemMessage(content)),
	}, nil
}

func (t *Template[T]) Message(ctx context.Context, vars T) (*schema.Message, error) {
	if t == nil || t.tmpl == nil {
		return nil, errors.New("prompt template is not initialized")
	}

	promptVars := vars.PromptVars()
	if promptVars == nil {
		promptVars = map[string]any{}
	}

	msgs, err := t.tmpl.Format(ctx, promptVars)
	if err != nil {
		return nil, fmt.Errorf("render prompt template %q for lang %q: %w", t.name, t.lang, err)
	}

	if len(msgs) != 1 {
		return nil, fmt.Errorf("expected one prompt message, got %d", len(msgs))
	}

	return msgs[0], nil
}

func readTemplate(name, lang string) (string, error) {
	file := path.Join(lang, name+".jinja")
	content, err := templateFiles.ReadFile(file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("prompt template %q for lang %q not found, supported langs: %s",
				name, lang, strings.Join(supportedLangs(name), ", "))
		}
		return "", fmt.Errorf("read prompt template %q: %w", file, err)
	}

	return string(content), nil
}

func supportedLangs(name string) []string {
	matches, err := fs.Glob(templateFiles, "*/"+name+".jinja")
	if err != nil {
		return nil
	}

	langs := make([]string, 0, len(matches))
	for _, match := range matches {
		langs = append(langs, path.Dir(match))
	}

	sort.Strings(langs)
	return langs
}
