package prompt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMindmap(t *testing.T) {
	msgs, err := RenderMindmap(context.Background(), []string{"src-1"})
	require.NoError(t, err)
	assert.NotEmpty(t, msgs)
	assert.Contains(t, msgs[len(msgs)-1].Content, "src-1")
}

func TestRenderReport(t *testing.T) {
	_, err := RenderReport(context.Background(), []string{"s1", "s2"})
	require.NoError(t, err)
}

func TestRenderInfographic(t *testing.T) {
	_, err := RenderInfographic(context.Background(), StudioInfoGraphicTemplateVars{
		SourceIds: []string{"s1"}, TextLanguage: "zh-cn", ExtraPrompt: "p", Orientation: "landscape", DetailLevel: "standard",
	})
	require.NoError(t, err)
}

func TestRenderTitleMaker(t *testing.T) {
	_, err := RenderTitleMaker(context.Background(), "report content")
	require.NoError(t, err)
}

func TestCheckStudioMindmapResult(t *testing.T) {
	good := "```mermaid\nmindmap\nroot((Root))\n  A\n```"
	assert.True(t, CheckStudioMindmapResult(good))

	bad := "not a mindmap"
	assert.False(t, CheckStudioMindmapResult(bad))
}
