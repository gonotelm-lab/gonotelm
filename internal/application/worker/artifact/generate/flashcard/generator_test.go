package flashcard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/generate/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

func TestRenderFlashcard(t *testing.T) {
	msgs, err := RenderFlashcard(
		t.Context(),
		[]string{"src-1"},
		artifactentity.FlashcardCountFew,
		artifactentity.FlashcardDifficultyEasy,
		"侧重所有权",
	)
	require.NoError(t, err)
	assert.NotEmpty(t, msgs)
	content := msgs[len(msgs)-1].Content
	assert.Contains(t, content, "src-1")
	assert.Contains(t, content, "侧重所有权")
	assert.Contains(t, content, "数量偏少")
	assert.Contains(t, content, "难度简单")
}

func TestGenerator_ImplementsGenerator(t *testing.T) {
	var _ types.Generator = New(nil)
}
