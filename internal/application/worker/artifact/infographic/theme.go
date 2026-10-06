package infographic

import (
	"embed"
	"strings"

	"github.com/bytedance/sonic"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

//go:embed skills/theme
var themeFS embed.FS

func visualTheme(style artifactentity.InfoGraphicVisualStyle) (themeJSON, styleClause string) {
	raw, ok := readThemeFile(style)
	if !ok {
		raw, _ = readThemeFile(artifactentity.InfoGraphicVisualStyleDefault)
	}

	var parsed struct {
		Guidance struct {
			Style string `json:"style"`
		} `json:"guidance"`
	}
	if err := sonic.Unmarshal(raw, &parsed); err != nil {
		return string(raw), ""
	}

	return strings.TrimRight(string(raw), "\n"), parsed.Guidance.Style
}

func readThemeFile(style artifactentity.InfoGraphicVisualStyle) ([]byte, bool) {
	if !style.Supported() {
		return nil, false
	}

	raw, err := themeFS.ReadFile("skills/theme/" + style.String() + "/theme.json")
	if err != nil {
		return nil, false
	}
	return raw, true
}
