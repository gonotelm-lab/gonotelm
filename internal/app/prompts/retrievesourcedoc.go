package prompts

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"
)

type RetrieveSource struct {
	// 来源id
	Id string

	// 来源名称
	Name string

	// 来源摘要
	Abstract string
}

type RetrieveSourceDocTemplateVars struct {
	Question   string
	NotebookId string
	Sources    []*RetrieveSource
}

func (v RetrieveSourceDocTemplateVars) PromptVars() map[string]any {
	return map[string]any{
		"Question":   strings.TrimSpace(v.Question),
		"NotebookId": strings.TrimSpace(v.NotebookId),
		"Sources":    normalizeRetrieveSources(v.Sources),
	}
}

type RetrieveSourceDocTemplate = template[RetrieveSourceDocTemplateVars]

func NewRetrieveSourceDocTemplate(lang string) *RetrieveSourceDocTemplate {
	return newTemplate[RetrieveSourceDocTemplateVars](templateNameRetrieveSourceDoc, lang)
}

func RenderRetrieveSourceDocMessage(
	ctx context.Context,
	question string,
	notebookId string,
	sources []*RetrieveSource,
	lang string,
) ([]*schema.Message, error) {
	tmpl := NewRetrieveSourceDocTemplate(lang)
	msg, err := tmpl.Message(ctx, RetrieveSourceDocTemplateVars{
		Question:   question,
		NotebookId: notebookId,
		Sources:    sources,
	})
	if err != nil {
		return nil, err
	}

	return prependSystemMessage([]*schema.Message{msg}), nil
}

func normalizeRetrieveSources(sources []*RetrieveSource) []*RetrieveSource {
	normalized := make([]*RetrieveSource, 0, len(sources))
	for _, source := range sources {
		if source == nil {
			continue
		}

		id := strings.TrimSpace(source.Id)
		name := strings.TrimSpace(source.Name)
		abstract := strings.TrimSpace(source.Abstract)

		if id == "" && name == "" && abstract == "" {
			continue
		}

		normalized = append(normalized, &RetrieveSource{
			Id:       id,
			Name:     name,
			Abstract: abstract,
		})
	}

	return normalized
}
