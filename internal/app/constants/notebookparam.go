package constants

import (
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

const (
	// max rune count
	MaxNotebookNameLength        = 128
	MaxNotebookDescriptionLength = 1024
)

func TruncateNotebookName(name string) string {
	return pkgstring.TruncateRune(name, MaxNotebookNameLength)
}

func TruncateNotebookDescription(description string) string {
	return pkgstring.TruncateRune(description, MaxNotebookDescriptionLength)
}
