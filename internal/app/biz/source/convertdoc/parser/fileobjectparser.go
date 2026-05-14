package parser

import (
	"context"
	"io"

	"github.com/gonotelm-lab/gonotelm/internal/app/model"

	docxparser "github.com/cloudwego/eino-ext/components/document/parser/docx"
	einoparser "github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	epubparser "github.com/gonotelm-lab/gonotelm/pkg/eino-ext/parser/epub"
	pdfmarkdownparser "github.com/gonotelm-lab/gonotelm/pkg/eino-ext/parser/pdf"
)

type FileObjectParser struct{}

var _ einoparser.Parser = (*FileObjectParser)(nil)

func (p *FileObjectParser) Parse(
	ctx context.Context,
	r io.Reader,
	opt ...einoparser.Option,
) ([]*schema.Document, error) {
	customOpts := &customParseOption{}
	einoparser.GetImplSpecificOptions(customOpts, opt...)

	return p.parseByMime(
		ctx,
		r,
		ResolveSourceMime(customOpts.fileMime, customOpts.fileExt),
		opt...,
	)
}

func (p *FileObjectParser) parseByMime(
	ctx context.Context,
	r io.Reader,
	mimeType string,
	opts ...einoparser.Option,
) ([]*schema.Document, error) {
	textParser := einoparser.TextParser{}

	switch mimeType {
	case model.MimeTypeText, model.MimeTypeMarkdown: // plain text or markdown is already just text itself
		return textParser.Parse(ctx, r, opts...)
	case model.MimeTypePDF:
		return parseByDocParser(
			ctx,
			r,
			pdfmarkdownparser.NewPDFParser(nil), // output will try to be markdown
			opts...,
		)
	case model.MimeTypeWord:
		wordParser, _ := docxparser.NewDocxParser(ctx,
			&docxparser.Config{
				IncludeTables: true,
			},
		)
		return parseByDocParser(
			ctx,
			r,
			wordParser,
			opts...,
		)
	case model.MimeTypeEPUB:
		return parseByDocParser(
			ctx,
			r,
			epubparser.NewEPUBParser(&epubparser.Config{
				OutputFormat: epubparser.OutputFormatMarkdown,
				ToPages:      false,
			}),
			opts...,
		)
	}

	// text parser fallback
	return textParser.Parse(ctx, r, opts...)
}

func parseByDocParser(
	ctx context.Context,
	r io.Reader,
	docParser einoparser.Parser,
	opts ...einoparser.Option,
) ([]*schema.Document, error) {
	return docParser.Parse(ctx, r, opts...)
}
