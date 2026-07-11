package prompt

import (
	"regexp"
	"strings"
)

var studioMindmapRootLineRegexp = regexp.MustCompile(`^\s*root\(\(.+\)\)\s*$`)

func normalizeStrings(sources []string) []string {
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

func normalizeOrientation(orientation string) string {
	normalized := strings.ToLower(strings.TrimSpace(orientation))
	switch normalized {
	case "portrait", "landscape", "square":
		return normalized
	default:
		return "landscape"
	}
}

func normalizeDetailLevel(level string) string {
	normalized := strings.ToLower(strings.TrimSpace(level))
	switch normalized {
	case "concise", "standard", "detailed":
		return normalized
	default:
		return "standard"
	}
}

func CheckStudioMindmapResult(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}

	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) < 3 {
		return false
	}

	if strings.TrimSpace(lines[0]) != "```mermaid" {
		return false
	}
	if strings.TrimSpace(lines[len(lines)-1]) != "```" {
		return false
	}

	bodyLines := lines[1 : len(lines)-1]
	nonEmptyBodyLines := make([]string, 0, len(bodyLines))
	for _, line := range bodyLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "```") {
			return false
		}
		nonEmptyBodyLines = append(nonEmptyBodyLines, line)
	}

	if len(nonEmptyBodyLines) < 2 {
		return false
	}

	if strings.TrimSpace(nonEmptyBodyLines[0]) != "mindmap" {
		return false
	}

	nodeLines := nonEmptyBodyLines[1:]
	if !studioMindmapRootLineRegexp.MatchString(nodeLines[0]) {
		return false
	}

	rootCount := 0
	for _, line := range nodeLines {
		if studioMindmapRootLineRegexp.MatchString(line) {
			rootCount++
		}
	}
	if rootCount != 1 {
		return false
	}

	return true
}
