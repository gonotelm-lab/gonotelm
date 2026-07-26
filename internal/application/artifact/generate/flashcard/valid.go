package flashcard

import (
	"strings"
)

type FlashcardCard struct {
	Front string `json:"front"`
	Back  string `json:"back"`
	Hint  string `json:"hint"`
}

type FlashcardContent struct {
	Cards []FlashcardCard `json:"cards"`
}

type flashcardExpectation struct {
	Title     string           `json:"title"`
	Flashcard FlashcardContent `json:"flashcard"`
}

func CheckFlashcardContent(content FlashcardContent) bool {
	if len(content.Cards) == 0 {
		return false
	}
	for _, card := range content.Cards {
		if strings.TrimSpace(card.Front) == "" || strings.TrimSpace(card.Back) == "" {
			return false
		}
	}
	return true
}
