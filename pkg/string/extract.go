package string

import "strings"

func ExtractBetweenTags(s, openTag, closeTag string) string {
	start := strings.Index(s, openTag)
	if start == -1 {
		return ""
	}

	contentStart := start + len(openTag)
	end := strings.Index(s[contentStart:], closeTag)
	if end == -1 {
		return ""
	}

	return s[contentStart : contentStart+end]
}
