package quiz

import (
	"fmt"
	"strings"
)

func CheckQuizContent(content QuizContent) bool {
	return ValidateQuizContent(content) == nil
}

func ValidateQuizContent(content QuizContent) error {
	if len(content.Questions) == 0 {
		return fmt.Errorf("questions must be non-empty")
	}
	if len(content.Themes) == 0 {
		return fmt.Errorf("themes must be non-empty")
	}
	if len(content.FollowUpHint) == 0 {
		return fmt.Errorf("follow_up_hint must be non-empty")
	}
	for i, theme := range content.Themes {
		if strings.TrimSpace(theme) == "" {
			return fmt.Errorf("themes[%d] is empty", i)
		}
	}
	for i, hint := range content.FollowUpHint {
		if strings.TrimSpace(hint) == "" {
			return fmt.Errorf("follow_up_hint[%d] is empty", i)
		}
	}

	seenMulti := false
	for qi, q := range content.Questions {
		if strings.TrimSpace(q.Question) == "" {
			return fmt.Errorf("questions[%d].question is empty", qi)
		}
		if strings.TrimSpace(q.Explanation) == "" {
			return fmt.Errorf("questions[%d].explanation is empty", qi)
		}
		if len(q.Options) != quizOptionCount {
			return fmt.Errorf("questions[%d].options must have exactly %d items, got %d", qi, quizOptionCount, len(q.Options))
		}
		for oi, opt := range q.Options {
			if strings.TrimSpace(opt) == "" {
				return fmt.Errorf("questions[%d].options[%d] is empty", qi, oi)
			}
		}
		if len(q.AnswerIndex) == 0 {
			return fmt.Errorf("questions[%d].answer_index must be non-empty", qi)
		}
		if len(q.AnswerIndex) > 1 {
			seenMulti = true
		} else if seenMulti {
			return fmt.Errorf("questions[%d] is single-choice but appears after multi-choice; singles must come first", qi)
		}
		seen := make(map[int]struct{}, len(q.AnswerIndex))
		for _, idx := range q.AnswerIndex {
			if idx < 0 || idx >= quizOptionCount {
				return fmt.Errorf("questions[%d].answer_index contains out-of-range index %d (valid: 0-%d)", qi, idx, quizOptionCount-1)
			}
			if _, ok := seen[idx]; ok {
				return fmt.Errorf("questions[%d].answer_index contains duplicate index %d", qi, idx)
			}
			seen[idx] = struct{}{}
		}
	}
	return nil
}
