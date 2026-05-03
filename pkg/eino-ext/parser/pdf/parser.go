package pdf

import (
	"context"
	"fmt"
	"io"
	"maps"

	einoparser "github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

const markdownContentType = "text/markdown"

type Config struct {
}

type PDFParser struct{}

var _ einoparser.Parser = (*PDFParser)(nil)

func NewPDFParser(config *Config) *PDFParser {
	return &PDFParser{}
}

func (p *PDFParser) Parse(
	ctx context.Context,
	reader io.Reader,
	opts ...einoparser.Option,
) ([]*schema.Document, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read pdf data failed: %w", err)
	}

	pageMarkdowns, err := extractPDFMarkdownPages(ctx, data)
	if err != nil {
		return nil, err
	}

	commonOpts := einoparser.GetCommonOptions(nil, opts...)
	baseMeta := copyMeta(commonOpts.ExtraMeta)
	baseMeta["content_type"] = markdownContentType
	if commonOpts.URI != "" {
		baseMeta[einoparser.MetaKeySource] = commonOpts.URI
	}

	return []*schema.Document{
		{
			ID:       uuid.NewV4().String(),
			Content:  buildPDFMergedMarkdown(pageMarkdowns),
			MetaData: copyMeta(baseMeta),
		},
	}, nil
}

func copyMeta(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}

	dst := make(map[string]any, len(src))
	maps.Copy(dst, src)
	return dst
}
