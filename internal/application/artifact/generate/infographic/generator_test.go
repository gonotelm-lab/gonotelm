package infographic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderInfographic(t *testing.T) {
	_, err := RenderInfographic(t.Context(), TemplateVars{
		SourceIds: []string{"s1"}, TextLanguage: "zh-cn", ExtraPrompt: "p", Orientation: "landscape", DetailLevel: "standard",
	})
	require.NoError(t, err)
}

func TestGenerator_ImplementsGenerator(t *testing.T) {
	var _ Generator = *New(nil)
}
