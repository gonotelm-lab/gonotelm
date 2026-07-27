package datatable

import pkgmarkdown "github.com/gonotelm-lab/gonotelm/pkg/string/markdown"

func NormalizeDataTableMarkdown(content string) (string, error) {
	return pkgmarkdown.NormalizeExclusivePipeTable(content)
}

func ValidateDataTableMarkdown(content string) error {
	return pkgmarkdown.ValidateExclusivePipeTable(content)
}
