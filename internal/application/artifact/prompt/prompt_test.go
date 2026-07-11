package prompt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrompt_RenderStudioMindmapV2(t *testing.T) {
	p := New("zh")
	msgs, err := p.RenderStudioMindmapV2Message(context.Background(), []string{"src-1"}, "")
	require.NoError(t, err)
	assert.NotEmpty(t, msgs)
	assert.Contains(t, msgs[len(msgs)-1].Content, "src-1")
}

func TestPrompt_RenderStudioReport(t *testing.T) {
	p := New("zh")
	_, err := p.RenderStudioReportMessage(context.Background(), []string{"s1", "s2"}, "zh")
	require.NoError(t, err)
}

func TestPrompt_RenderStudioInfoGraphic(t *testing.T) {
	p := New("zh")
	_, err := p.RenderStudioInfoGraphicMessage(context.Background(), StudioInfoGraphicTemplateVars{
		SourceIds: []string{"s1"}, TextLanguage: "zh-cn", ExtraPrompt: "p", Orientation: "landscape", DetailLevel: "standard",
	}, "")
	require.NoError(t, err)
}

func TestPrompt_RenderTitleMaker(t *testing.T) {
	p := New("zh")
	_, err := p.RenderTitleMakerMessage(context.Background(), "report content", "")
	require.NoError(t, err)
}

func TestCheckStudioMindmapResult(t *testing.T) {
	good := "```mermaid\nmindmap\nroot((Root))\n  A\n```"
	assert.True(t, CheckStudioMindmapResult(good))

	bad := "not a mindmap"
	assert.False(t, CheckStudioMindmapResult(bad))
}
