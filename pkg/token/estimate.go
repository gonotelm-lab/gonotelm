package token

import "unicode"

var asciiTokenWeight [128]float64

func init() {
	for i := range 128 {
		asciiTokenWeight[i] = slowRuneWeight(rune(i))
	}
}

func slowRuneWeight(r rune) float64 {
	switch {
	case unicode.Is(unicode.Han, r):
		return 1.0 / 1.2
	case unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r):
		return 1.0 / 1.5
	case unicode.Is(unicode.Cyrillic, r):
		return 1.0 / 3.0
	case unicode.Is(unicode.Arabic, r):
		return 1.0 / 2.5
	case unicode.Is(unicode.Latin, r):
		return 1.0 / 3.5
	case unicode.IsDigit(r):
		return 1.0 / 4.0
	case unicode.Is(unicode.So, r) || unicode.Is(unicode.Sk, r) || unicode.Is(unicode.Sm, r):
		return 1.0
	case unicode.IsPunct(r):
		return 1.0 / 2.0
	case unicode.IsSpace(r):
		return 1.0 / 5.0
	default:
		return 1.0 / 2.0
	}
}

// Simple token estimation
func Estimate(text string) int {
	if text == "" {
		return 0
	}

	var tokens float64
	for _, r := range text {
		if r < 128 {
			tokens += asciiTokenWeight[r]
			continue
		}
		// CJK Unified Ideographs (most common Han characters)
		if r >= 0x4E00 && r <= 0x9FFF {
			tokens += 1.0 / 1.2
			continue
		}
		// Common Hiragana / Katakana
		if r >= 0x3040 && r <= 0x30FF {
			tokens += 1.0 / 1.5
			continue
		}
		// Common Hangul Syllables
		if r >= 0xAC00 && r <= 0xD7AF {
			tokens += 1.0 / 1.5
			continue
		}
		tokens += slowRuneWeight(r)
	}

	return int(tokens) + 1
}
