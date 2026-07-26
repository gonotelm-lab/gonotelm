package types

import (
	"strings"

	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

const MaxTitleLength = 128

// NormalizeTitle trims whitespace and truncates title to MaxTitleLength runes.
func NormalizeTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	return strings.TrimSpace(pkgstring.TruncateRune(title, MaxTitleLength))
}
