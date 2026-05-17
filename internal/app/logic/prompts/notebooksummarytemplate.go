package prompts

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"
)

type NotebookSummaryTemplateVars struct {
	Sources []string
}

func (v NotebookSummaryTemplateVars) PromptVars() map[string]any {
	return map[string]any{
		"sources": normalizeNotebookSummarySources(v.Sources),
	}
}

type NotebookSummaryTemplate = template[NotebookSummaryTemplateVars]

func NewNotebookSummaryTemplate(lang string) (*NotebookSummaryTemplate) {
	return newTemplate[NotebookSummaryTemplateVars](templateNameNotebookSummary, lang)
}

func NotebookSummaryPromptMessage(
	ctx context.Context,
	sources []string,
	lang string,
) (*schema.Message, error) {
	tmpl := NewNotebookSummaryTemplate(lang)
	msg, err := tmpl.Message(ctx, NotebookSummaryTemplateVars{Sources: sources})
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func NotebookSummaryPrompt(sources []string, lang string) (string, error) {
	msg, err := NotebookSummaryPromptMessage(context.Background(), sources, lang)
	if err != nil {
		return "", err
	}

	return msg.Content, nil
}

func normalizeNotebookSummarySources(sources []string) []string {
	normalized := make([]string, 0, len(sources))
	for _, source := range sources {
		text := strings.TrimSpace(source)
		if text == "" {
			continue
		}
		normalized = append(normalized, text)
	}

	return normalized
}
