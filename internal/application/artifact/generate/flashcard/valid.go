package flashcard

import (
	"strings"
)

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
