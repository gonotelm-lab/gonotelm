package quiz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAgentOutput_Valid(t *testing.T) {
	raw := `{
  "title": "Rust所有权测验题组示例",
  "quiz": {
    "questions": [
      {
        "question": "什么是所有权？",
        "options": ["A", "B", "C", "D"],
        "answer_index": [0],
        "explanation": "所有权是 Rust 管理内存的核心规则。"
      },
      {
        "question": "借用规则包括哪些？",
        "options": ["A", "B", "C", "D"],
        "answer_index": [0, 1],
        "explanation": "不可变借用可多个，可变借用同时最多一个。"
      }
    ],
    "themes": ["所有权", "借用"],
    "follow_up_hint": ["生命周期", "智能指针"]
  }
}`
	got, err := parseAgentOutput(t.Context(), raw)
	require.NoError(t, err)
	assert.Equal(t, "Rust所有权测验题组示例", got.Title)
	require.Len(t, got.Quiz.Questions, 2)
	assert.Equal(t, []int{0}, got.Quiz.Questions[0].AnswerIndex)
	assert.Equal(t, []int{0, 1}, got.Quiz.Questions[1].AnswerIndex)
}

func TestParseAgentOutput_RejectsWrongOptionCount(t *testing.T) {
	raw := `{
  "title": "选项数量错误测验示例",
  "quiz": {
    "questions": [
      {
        "question": "q",
        "options": ["A", "B", "C"],
        "answer_index": [0],
        "explanation": "解析"
      }
    ],
    "themes": ["t"],
    "follow_up_hint": ["h"]
  }
}`
	_, err := parseAgentOutput(t.Context(), raw)
	require.Error(t, err)
}

func TestParseAgentOutput_RejectsMultiBeforeSingle(t *testing.T) {
	raw := `{
  "title": "题型顺序错误测验示例",
  "quiz": {
    "questions": [
      {
        "question": "multi",
        "options": ["A", "B", "C", "D"],
        "answer_index": [0, 1],
        "explanation": "多选解析"
      },
      {
        "question": "single",
        "options": ["A", "B", "C", "D"],
        "answer_index": [0],
        "explanation": "单选解析"
      }
    ],
    "themes": ["t"],
    "follow_up_hint": ["h"]
  }
}`
	_, err := parseAgentOutput(t.Context(), raw)
	require.Error(t, err)
}

func TestCheckQuizContent(t *testing.T) {
	assert.True(t, CheckQuizContent(QuizContent{
		Questions: []QuizQuestion{{
			Question:    "q",
			Options:     []string{"A", "B", "C", "D"},
			AnswerIndex: []int{0},
			Explanation: "解析说明",
		}},
		Themes:       []string{"t"},
		FollowUpHint: []string{"h"},
	}))
	assert.False(t, CheckQuizContent(QuizContent{}))
	assert.False(t, CheckQuizContent(QuizContent{
		Questions: []QuizQuestion{{
			Question:    "q",
			Options:     []string{"A", "B", "C", "D"},
			AnswerIndex: []int{0},
			Explanation: "  ",
		}},
		Themes:       []string{"t"},
		FollowUpHint: []string{"h"},
	}))
}

func TestValidateQuizContent_OptionCountAndIndexBounds(t *testing.T) {
	err := ValidateQuizContent(QuizContent{
		Questions: []QuizQuestion{{
			Question:    "q",
			Options:     []string{"A", "B", "C"},
			AnswerIndex: []int{0},
			Explanation: "解析",
		}},
		Themes:       []string{"t"},
		FollowUpHint: []string{"h"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly 4")

	err = ValidateQuizContent(QuizContent{
		Questions: []QuizQuestion{{
			Question:    "q",
			Options:     []string{"A", "B", "C", "D"},
			AnswerIndex: []int{4},
			Explanation: "解析",
		}},
		Themes:       []string{"t"},
		FollowUpHint: []string{"h"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out-of-range")
}
