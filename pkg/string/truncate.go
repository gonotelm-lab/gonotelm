package string

func TruncateRune(s string, maxRuneCount int) string {
	if maxRuneCount <= 0 {
		return ""
	}

	runes := []rune(s)
	if len(runes) <= maxRuneCount {
		return s
	}

	return string(runes[:maxRuneCount])
}
