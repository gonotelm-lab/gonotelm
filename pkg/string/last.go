package string

import (
	"unicode/utf8"
)

func LastRune(s string, runeCount int) string {
	if s == "" {
		return s
	}

	// abcdefg
	len := utf8.RuneCountInString(s)
	targetIdx := max(0, min(len, len-runeCount))
	return string([]rune(s)[targetIdx:])
}
