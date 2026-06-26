package prompt

import (
	"strings"
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
