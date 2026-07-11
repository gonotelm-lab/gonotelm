package types

import "strings"

func NormalizeStrings(sources []string) []string {
	normalized := make([]string, 0, len(sources))
	for _, source := range sources {
		text := strings.TrimSpace(source)
		if text == "" {
			continue
		}
		normalized = append(normalized, text)
	}

	return normalized
}
