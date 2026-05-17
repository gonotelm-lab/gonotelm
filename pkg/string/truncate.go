package string

import "unicode/utf8"

func TruncateRune(s string, maxRuneCount int) string {
	if maxRuneCount <= 0 {
		return ""
	}

	runeCount := utf8.RuneCountInString(s)
	if runeCount <= maxRuneCount {
		return s
	}

	start := 0
	end := len(s)
	var result []byte = make([]byte, 0, runeCount*utf8.UTFMax)
	for range maxRuneCount {
		if runeCount > maxRuneCount {
			char, size := utf8.DecodeRuneInString(s[start:end])
			if char == utf8.RuneError {
				return ""
			}
			result = utf8.AppendRune(result, char)
			start += size
		}
	}

	return FromBytes(result)
}
