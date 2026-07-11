package report

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderReport(t *testing.T) {
	_, err := RenderReport(t.Context(), []string{"s1", "s2"})
	require.NoError(t, err)
}

func TestRenderTitleMaker(t *testing.T) {
	_, err := RenderTitleMaker(t.Context(), "report content")
	require.NoError(t, err)
}

func TestGenerator_ImplementsGenerator(t *testing.T) {
	var _ Generator = *New(nil)
}
