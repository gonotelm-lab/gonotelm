package infographic

import (
	"testing"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"

	"github.com/stretchr/testify/require"
)

func TestRenderInfographic(t *testing.T) {
	_, err := RenderInfographic(t.Context(), TemplateVars{
		SourceIds:    []string{"s1"},
		TextLanguage: "zh-cn",
		ExtraPrompt:  "p",
		Orientation:  artifactentity.ArtifactInfoGraphicOrientationLandscape,
		DetailLevel:  artifactentity.ArtifactInfoGraphicDetailLevelStandard,
	})
	require.NoError(t, err)
}

func TestGenerator_ImplementsGenerator(t *testing.T) {
	var _ Generator = *New(nil)
}
