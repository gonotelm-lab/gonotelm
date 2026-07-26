package markdown

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	pipeTableSeparatorCellRegexp = regexp.MustCompile(`^\s*:?-{3,}:?\s*$`)
	outerFenceOpenRegexp         = regexp.MustCompile("(?s)^\\s*```(?:markdown|md)?\\s*\\n(.*)\\n```\\s*$")
)

// NormalizeExclusivePipeTable trims content, optionally strips a single outer
// markdown fence, and returns the normalized exclusive GFM pipe table text.
func NormalizeExclusivePipeTable(content string) (string, error) {
	normalized, err := normalizeExclusivePipeTable(content)
	if err != nil {
		return "", err
	}
	return normalized, nil
}

// ValidateExclusivePipeTable reports whether content is exclusively one GFM
// pipe table (header + separator + >=1 data row), with no other elements.
func ValidateExclusivePipeTable(content string) error {
	_, err := normalizeExclusivePipeTable(content)
	return err
}

func normalizeExclusivePipeTable(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", fmt.Errorf("markdown table is empty")
	}

	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	if stripped, ok := stripOuterMarkdownFence(content); ok {
		content = strings.TrimSpace(stripped)
	}
	if content == "" {
		return "", fmt.Errorf("markdown table is empty")
	}
	if strings.Contains(content, "```") {
		return "", fmt.Errorf("markdown table must not contain code fences")
	}

	rawLines := strings.Split(content, "\n")
	lines := make([]string, 0, len(rawLines))
	for i, line := range rawLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(lines) == 0 {
				continue
			}
			// Trailing blanks are ignored; blanks inside the table are rejected.
			hasMore := false
			for _, rest := range rawLines[i+1:] {
				if strings.TrimSpace(rest) != "" {
					hasMore = true
					break
				}
			}
			if !hasMore {
				break
			}
			return "", fmt.Errorf("markdown table must not contain blank lines")
		}
		lines = append(lines, trimmed)
	}

	if len(lines) < 3 {
		return "", fmt.Errorf("markdown table requires header, separator, and at least one data row")
	}

	colCount := 0
	for i, line := range lines {
		cells, err := splitPipeTableRow(line)
		if err != nil {
			return "", fmt.Errorf("line %d: %w", i+1, err)
		}
		if i == 0 {
			if len(cells) == 0 {
				return "", fmt.Errorf("table header must have at least one column")
			}
			colCount = len(cells)
			continue
		}
		if len(cells) != colCount {
			return "", fmt.Errorf("line %d: column count %d does not match header %d", i+1, len(cells), colCount)
		}
		if i == 1 {
			if !isPipeTableSeparatorRow(cells) {
				return "", fmt.Errorf("line 2 must be a markdown table separator row")
			}
		}
	}

	return strings.Join(lines, "\n") + "\n", nil
}

func stripOuterMarkdownFence(content string) (string, bool) {
	matches := outerFenceOpenRegexp.FindStringSubmatch(content)
	if len(matches) != 2 {
		return "", false
	}
	return matches[1], true
}

func splitPipeTableRow(line string) ([]string, error) {
	if !strings.Contains(line, "|") {
		return nil, fmt.Errorf("not a markdown table row")
	}

	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")

	parts := strings.Split(trimmed, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	if len(cells) == 0 {
		return nil, fmt.Errorf("table row has no cells")
	}
	return cells, nil
}

func isPipeTableSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		if !pipeTableSeparatorCellRegexp.MatchString(cell) {
			return false
		}
	}
	return true
}
