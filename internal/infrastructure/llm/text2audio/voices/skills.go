package voices

import (
	_ "embed"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
)

//go:embed qwen.md
var qwenSkill string

//go:embed mimo.md
var mimoSkill string

func GetProviderSkill(provider text2audio.Text2AudioProvider) string {
	switch provider {
	case text2audio.Text2AudioQwen:
		return qwenSkill
	case text2audio.Text2AudioMimo:
		return mimoSkill
	default:
		return ""
	}
}
