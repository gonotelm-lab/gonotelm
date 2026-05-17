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

type templateName string

const (
	defaultLang = "zh"

	templateNameChat            templateName = "chat"
	templateNameSummarize       templateName = "summarize"
	templateNameNotebookSummary templateName = "notebook-summary"
)

//go:embed */*.jinja
var templateFiles embed.FS

var templateStore = loadTemplateStore(templateFiles)

type preloadedTemplates struct {
	contentByFile map[string]string
	langsByName   map[templateName][]string
}

func loadTemplateStore(fsys fs.FS) preloadedTemplates {
	store := preloadedTemplates{
		contentByFile: map[string]string{},
		langsByName:   map[templateName][]string{},
	}

	files, err := fs.Glob(fsys, "*/*.jinja")
	if err != nil {
		panic(fmt.Sprintf("glob prompt templates failed: %v", err))
	}

	for _, file := range files {
		content, err := fs.ReadFile(fsys, file)
		if err != nil {
			panic(fmt.Sprintf("read prompt template %q failed: %v", file, err))
		}

		store.contentByFile[file] = string(content)
		templateName := templateName(strings.TrimSuffix(path.Base(file), path.Ext(file)))
		store.langsByName[templateName] = append(store.langsByName[templateName], path.Dir(file))
	}

	for name := range store.langsByName {
		sort.Strings(store.langsByName[name])
	}

	return store
}

type templateVars interface {
	PromptVars() map[string]any
}

type template[T templateVars] struct {
	name templateName
	lang string
	tmpl prompt.ChatTemplate
}

func newTemplate[T templateVars](tmplName templateName, lang string) *template[T] {
	normalizedName := normalizeTemplateName(tmplName)
	normalizedLang := normalizeTemplateLang(lang)
	content := readTemplate(normalizedName, normalizedLang)

	return &template[T]{
		name: normalizedName,
		lang: normalizedLang,
		tmpl: prompt.FromMessages(schema.Jinja2, schema.SystemMessage(content)),
	}
}

func (t *template[T]) Message(ctx context.Context, vars T) (*schema.Message, error) {
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

func readTemplate(name templateName, lang string) string {
	file := path.Join(lang, string(name)+".jinja")
	content, ok := templateStore.contentByFile[file]
	if ok {
		return content
	}

	defaultFile := path.Join(defaultLang, string(name)+".jinja")
	if content, ok := templateStore.contentByFile[defaultFile]; ok {
		return content
	}

	langs := supportedLangs(name)
	if len(langs) == 0 {
		panic(fmt.Sprintf("prompt template %q is not initialized", name))
	}

	fallbackFile := path.Join(langs[0], string(name)+".jinja")
	content, ok = templateStore.contentByFile[fallbackFile]
	if !ok {
		panic(fmt.Sprintf("prompt template %q fallback for lang %q is not initialized", name, langs[0]))
	}

	return content
}

func supportedLangs(name templateName) []string {
	langs := templateStore.langsByName[name]
	if len(langs) == 0 {
		return nil
	}

	cloned := make([]string, len(langs))
	copy(cloned, langs)
	return cloned
}

func normalizeTemplateName(name templateName) templateName {
	normalizedName := templateName(strings.TrimSpace(string(name)))
	switch normalizedName {
	case templateNameChat, templateNameSummarize, templateNameNotebookSummary:
		return normalizedName
	case "":
		panic("template name is required")
	default:
		panic(fmt.Sprintf("unsupported template name: %s", normalizedName))
	}
}
