package voices

import (
	_ "embed"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
)

//go:embed dashscope.md
var dashscopeSkill string

//go:embed mimo.md
var mimoSkill string

func GetProviderSkill(provider text2audio.Text2AudioProvider) string {
	switch provider {
	case text2audio.Text2AudioDashScope:
		return dashscopeSkill
	case text2audio.Text2AudioMimo:
		return mimoSkill
	default:
		return ""
	}
}
