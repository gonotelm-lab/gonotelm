package xlsx

import (
	"context"
	"fmt"
	"io"
	"strings"

	einoparser "github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	"github.com/xuri/excelize/v2"
)

// Config configures XLSXParser.
type Config struct {
	// IncludeCharts extracts chart series via chart XML metadata (not OCR).
	// Default true when cfg is nil or field left zero with useDefaults.
	IncludeCharts *bool
}

// XLSXParser converts an xlsx workbook into a single markdown Document.
type XLSXParser struct {
	includeCharts bool
}

// NewXLSXParser creates a parser. cfg may be nil (charts included by default).
func NewXLSXParser(cfg *Config) *XLSXParser {
	include := true
	if cfg != nil && cfg.IncludeCharts != nil {
		include = *cfg.IncludeCharts
	}
	return &XLSXParser{includeCharts: include}
}

var _ einoparser.Parser = (*XLSXParser)(nil)

func (p *XLSXParser) Parse(
	ctx context.Context,
	reader io.Reader,
	opts ...einoparser.Option,
) ([]*schema.Document, error) {
	commonOpts := einoparser.GetCommonOptions(nil, opts...)

	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, fmt.Errorf("xlsx open: %w", err)
	}
	defer func() { _ = f.Close() }()

	var b strings.Builder
	sheets := f.GetSheetList()
	for i, name := range sheets {
		if i > 0 {
			b.WriteByte('\n')
		}
		if err := writeSheetMarkdown(ctx, &b, f, name, p.includeCharts); err != nil {
			return nil, err
		}
	}
	writeOrphanCharts(&b, f, p.includeCharts)

	meta := map[string]any{
		"content_type": "text/markdown",
	}
	if commonOpts != nil {
		if commonOpts.URI != "" {
			meta[einoparser.MetaKeySource] = commonOpts.URI
		}
		for k, v := range commonOpts.ExtraMeta {
			meta[k] = v
		}
	}

	return []*schema.Document{{
		Content:  b.String(),
		MetaData: meta,
	}}, nil
}
