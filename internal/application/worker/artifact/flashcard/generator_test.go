package flashcard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"

	einoschema "github.com/cloudwego/eino/schema"
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
	require.Len(t, msgs, 2)
	assert.Contains(t, msgs[0].Content, "src-1")
	assert.Contains(t, msgs[0].Content, "数量偏少")
	assert.Contains(t, msgs[0].Content, "难度简单")
	assert.Equal(t, einoschema.User, msgs[1].Role)
	assert.Contains(t, msgs[1].Content, "侧重所有权")
}

func TestGenerator_ImplementsGenerator(t *testing.T) {
	var _ types.Generator = New(nil)
}
