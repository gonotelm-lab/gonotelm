package prompt

import (
	"fmt"
	"strings"
)

type SummarizeTemplateVars struct {
	Text    string
	MaxWord int
	MinWord int
}

func (v SummarizeTemplateVars) PromptVars() map[string]any {
	if v.MaxWord <= 0 {
		v.MaxWord = 150
	}
	if v.MinWord <= 0 {
		v.MinWord = 60
	}

	wordRange := fmt.Sprintf("%d-%d", v.MinWord, v.MaxWord)

	if v.MaxWord == v.MinWord {
		wordRange = fmt.Sprintf("%d", v.MinWord)
	}
	return map[string]any{
		"WordRange": wordRange,
		"Text":      strings.TrimSpace(v.Text),
	}
}

type SummarizeTemplate = template[SummarizeTemplateVars]
