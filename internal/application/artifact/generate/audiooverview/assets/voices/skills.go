package voices

import (
	_ "embed"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
)

//go:embed dashscope.md
var dashscopeSkill string

func GetProviderSkill(provider text2audio.Text2AudioProvider) string {
	switch provider {
	case text2audio.Text2AudioDashScope:
		return dashscopeSkill
	default:
		return ""
	}
}
