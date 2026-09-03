package string

import "strconv"

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

func TruncateRuneV2(s string, maxRuneCount int) (string, bool) {
	if maxRuneCount <= 0 {
		return "", false
	}

	runes := []rune(s)
	if len(runes) <= maxRuneCount {
		return s, false
	}

	return string(runes[:maxRuneCount]), true
}

// TruncateHeadTail keeps the head and tail of s for logs; middle is omitted with a length marker.
func TruncateHeadTail(s string, head, tail int) string {
	if head < 0 {
		head = 0
	}
	if tail < 0 {
		tail = 0
	}
	runes := []rune(s)
	n := len(runes)
	if n <= head+tail {
		return s
	}
	return string(runes[:head]) +
		"(... truncated, total_runes=" + strconv.Itoa(n) + " ...) " +
		string(runes[n-tail:])
}
