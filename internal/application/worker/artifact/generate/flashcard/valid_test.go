package flashcard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAgentOutput_Valid(t *testing.T) {
	raw := `{
  "title": "Rust所有权核心闪卡合集",
  "flashcard": {
    "cards": [
      {
        "front": "什么是所有权？",
        "back": "每个值有唯一所有者。",
        "hint": "想想谁负责释放内存"
      }
    ]
  }
}`
	got, err := parseAgentOutput(t.Context(), raw)
	require.NoError(t, err)
	assert.Equal(t, "Rust所有权核心闪卡合集", got.Title)
	require.Len(t, got.Flashcard.Cards, 1)
	assert.Equal(t, "什么是所有权？", got.Flashcard.Cards[0].Front)
	assert.Equal(t, "每个值有唯一所有者。", got.Flashcard.Cards[0].Back)
	assert.Equal(t, "想想谁负责释放内存", got.Flashcard.Cards[0].Hint)
}

func TestParseAgentOutput_RejectsEmptyCards(t *testing.T) {
	raw := `{"title":"空闪卡标题示例文本","flashcard":{"cards":[]}}`
	_, err := parseAgentOutput(t.Context(), raw)
	require.Error(t, err)
}

func TestParseAgentOutput_RejectsMissingFront(t *testing.T) {
	raw := `{
  "title": "字段缺失闪卡标题示例",
  "flashcard": {
    "cards": [
      {"front":"", "back":"答案", "hint":""}
    ]
  }
}`
	_, err := parseAgentOutput(t.Context(), raw)
	require.Error(t, err)
}

func TestCheckFlashcardContent(t *testing.T) {
	assert.True(t, CheckFlashcardContent(FlashcardContent{
		Cards: []FlashcardCard{{Front: "q", Back: "a", Hint: "h"}},
	}))
	assert.False(t, CheckFlashcardContent(FlashcardContent{Cards: nil}))
	assert.False(t, CheckFlashcardContent(FlashcardContent{
		Cards: []FlashcardCard{{Front: "", Back: "a"}},
	}))
}
