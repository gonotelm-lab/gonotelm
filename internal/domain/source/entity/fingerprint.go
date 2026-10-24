package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Fingerprint 单源指纹材料；title/abstract 皆空时返回空串。
func (s *Source) Fingerprint() string {
	if s == nil {
		return ""
	}
	title := strings.TrimSpace(s.Title)
	abstract := strings.TrimSpace(s.Abstract)
	if title == "" && abstract == "" {
		return ""
	}
	return fmt.Sprintf("%s\n%s\n%s", s.Id.String(), title, abstract)
}

// NewFingerprint Ready 且有内容的源集合指纹。
func NewFingerprint(sources []*Source) string {
	parts := make([]string, 0, len(sources))
	for _, source := range sources {
		if source == nil || !source.Status.IsReady() {
			continue
		}
		if fp := source.Fingerprint(); fp != "" {
			parts = append(parts, fp)
		}
	}
	sort.Strings(parts)

	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p)
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
