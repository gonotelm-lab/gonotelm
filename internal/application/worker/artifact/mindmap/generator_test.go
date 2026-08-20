package mindmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckStudioMindmapResult(t *testing.T) {
	good := "```mermaid\nmindmap\nroot((Root))\n  A\n```"
	assert.True(t, CheckStudioMindmapResult(good))

	bad := "not a mindmap"
	assert.False(t, CheckStudioMindmapResult(bad))
}

func TestRenderMindmap(t *testing.T) {
	msgs, err := RenderMindmap(t.Context(), []string{"src-1"}, "")
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Contains(t, msgs[0].Content, "src-1")
	assert.Equal(t, "user", string(msgs[1].Role))
}

func TestGenerator_ImplementsGenerator(t *testing.T) {
	var _ = *New(nil)
}
