package entity

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type FlashcardCount string

const (
	FlashcardCountFew     FlashcardCount = "few"
	FlashcardCountDefault FlashcardCount = "default"
	FlashcardCountMany    FlashcardCount = "many"
)

func (c FlashcardCount) String() string { return string(c) }

func (c FlashcardCount) Supported() bool {
	switch c {
	case FlashcardCountFew, FlashcardCountDefault, FlashcardCountMany:
		return true
	}
	return false
}

func FlashcardCountDefaultValue() FlashcardCount {
	return FlashcardCountDefault
}

type FlashcardDifficulty string

const (
	FlashcardDifficultyEasy   FlashcardDifficulty = "easy"
	FlashcardDifficultyMedium FlashcardDifficulty = "medium"
	FlashcardDifficultyHard   FlashcardDifficulty = "hard"
)

func (d FlashcardDifficulty) String() string { return string(d) }

func (d FlashcardDifficulty) Supported() bool {
	switch d {
	case FlashcardDifficultyEasy, FlashcardDifficultyMedium, FlashcardDifficultyHard:
		return true
	}
	return false
}

func FlashcardDifficultyDefault() FlashcardDifficulty {
	return FlashcardDifficultyMedium
}

type FlashcardPayload struct {
	NotebookId valobj.Id           `json:"notebook_id"`
	SourceIds  []valobj.Id         `json:"source_ids"`
	Count      FlashcardCount      `json:"count"`
	Difficulty FlashcardDifficulty `json:"difficulty"`
	Tip        string              `json:"tip"`
}

func (p *FlashcardPayload) Kind() Kind                { return KindFlashcard }
func (p *FlashcardPayload) GetSourceIds() []valobj.Id { return p.SourceIds }
