package entity

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type QuizCount string

const (
	QuizCountFew     QuizCount = "few"
	QuizCountDefault QuizCount = "default"
	QuizCountMany    QuizCount = "many"
)

func (c QuizCount) String() string { return string(c) }

func (c QuizCount) Supported() bool {
	switch c {
	case QuizCountFew, QuizCountDefault, QuizCountMany:
		return true
	}
	return false
}

func QuizCountDefaultValue() QuizCount {
	return QuizCountDefault
}

type QuizDifficulty string

const (
	QuizDifficultyEasy   QuizDifficulty = "easy"
	QuizDifficultyMedium QuizDifficulty = "medium"
	QuizDifficultyHard   QuizDifficulty = "hard"
)

func (d QuizDifficulty) String() string { return string(d) }

func (d QuizDifficulty) Supported() bool {
	switch d {
	case QuizDifficultyEasy, QuizDifficultyMedium, QuizDifficultyHard:
		return true
	}
	return false
}

func QuizDifficultyDefault() QuizDifficulty {
	return QuizDifficultyMedium
}

type QuizPayload struct {
	NotebookId valobj.Id      `json:"notebook_id"`
	SourceIds  []valobj.Id    `json:"source_ids"`
	Count      QuizCount      `json:"count"`
	Difficulty QuizDifficulty `json:"difficulty"`
	Tip        string         `json:"tip"`
}

func (p *QuizPayload) Kind() Kind                { return KindQuiz }
func (p *QuizPayload) GetSourceIds() []valobj.Id { return p.SourceIds }
