package prompt

import (
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
)

type NotebookSummaryTemplateVars struct {
	Sources    []string
	MaxNameLen int // rune count
	MaxDescLen int
}

func (v NotebookSummaryTemplateVars) PromptVars() map[string]any {
	if v.MaxNameLen <= 0 {
		v.MaxNameLen = constants.MaxNotebookNameLength
	}
	if v.MaxDescLen <= 0 {
		v.MaxDescLen = constants.MaxNotebookDescriptionLength
	}

	return map[string]any{
		"Sources":    normalizeStrings(v.Sources),
		"MaxNameLen": v.MaxNameLen,
		"MaxDescLen": v.MaxDescLen,
	}
}

type NotebookSummaryTemplate = template[NotebookSummaryTemplateVars]

func normalizeStrings(sources []string) []string {
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
