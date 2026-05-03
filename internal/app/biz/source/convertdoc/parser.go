package convertdoc

import (
	"context"
	"io"
	"mime"

	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	epubparser "github.com/gonotelm-lab/gonotelm/pkg/eino-ext/parser/epub"
	pdfmarkdownparser "github.com/gonotelm-lab/gonotelm/pkg/eino-ext/parser/pdf"

	docxparser "github.com/cloudwego/eino-ext/components/document/parser/docx"
	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
)

const (
	parserMetaSourceObjKey        = "_gonotelm_source_obj"
	parserMetaSourceIdKey         = "_gonotelm_source_id"
	parserMetaSourceNotebookIdKey = "_gonotelm_source_notebook_id"
	parserMetaSourceKindKey       = "_gonotelm_source_kind"

	chunkSplitMethodKey = "_gonotelm_chunk_split_method"
)

const (
	chunkHtmlSplitMethod      = "html"
	chunkMarkdownSplitMethod  = "markdown"
	chunkRecursiveSplitMethod = "recursive"
)

func attachChunkSplitMethod(doc *schema.Document, method string) *schema.Document {
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]any)
	}
	doc.MetaData[chunkSplitMethodKey] = method
	return doc
}

func getChunkSplitMethod(doc *schema.Document) string {
	if doc.MetaData == nil {
		return chunkRecursiveSplitMethod
	}
	method, ok := doc.MetaData[chunkSplitMethodKey]
	if !ok {
		return chunkRecursiveSplitMethod
	}

	if method, ok := method.(string); ok {
		return method
	}

	return chunkRecursiveSplitMethod
}

type customParseOption struct {
	source   *model.Source // 调用时必须提供
	fileMime string        // 文件来源时的mime type
	fileExt  string        // 文件来源时的format
}

func withParseSource(s *model.Source) parser.Option {
	return parser.WrapImplSpecificOptFn(func(t *customParseOption) {
		t.source = s
	})
}

func withParseFileMime(mimeType string) parser.Option {
	return parser.WrapImplSpecificOptFn(func(t *customParseOption) {
		t.fileMime = mimeType
	})
}

func withParseFileExt(ext string) parser.Option {
	return parser.WrapImplSpecificOptFn(func(t *customParseOption) {
		t.fileExt = ext
	})
}

type fileObjectParser struct{}

var _ parser.Parser = (*fileObjectParser)(nil)

func (p *fileObjectParser) Parse(
	ctx context.Context,
	r io.Reader,
	opt ...parser.Option,
) ([]*schema.Document, error) {
	customOpts := &customParseOption{}
	parser.GetImplSpecificOptions(customOpts, opt...)

	// try parse with mime type first, then try with ext parser
	return p.parseWithMime(
		ctx,
		r,
		customOpts.fileMime,
		customOpts.fileExt,
		opt...)
}

func (p *fileObjectParser) parseWithMime(
	ctx context.Context,
	r io.Reader,
	mimeType string,
	ext string,
	opts ...parser.Option,
) ([]*schema.Document, error) {
	var (
		sourceMime = mimeType
		textParser = parser.TextParser{}
	)

	if sourceMime == "" {
		mediaType := mime.TypeByExtension(ext)
		extMime, _, err := mime.ParseMediaType(mediaType)
		if err == nil {
			sourceMime = extMime
		}
	}

	switch sourceMime {
	case model.MimeTypeText, model.MimeTypeMarkdown: // plain text or markdown is already just text itself
		return textParser.Parse(ctx, r, opts...)
	case model.MimeTypePDF:
		p := pdfmarkdownparser.NewPDFParser(nil) // output will try to be markdown
		docs, err := p.Parse(ctx, r, opts...)
		if err != nil {
			return nil, err
		}
		for _, doc := range docs {
			attachChunkSplitMethod(doc, chunkMarkdownSplitMethod)
		}
		return docs, nil
	case model.MimeTypeWord:
		parser, _ := docxparser.NewDocxParser(ctx,
			&docxparser.Config{
				IncludeTables: true,
			})
		docs, err := parser.Parse(ctx, r, opts...)
		if err != nil {
			return nil, err
		}
		for _, doc := range docs {
			attachChunkSplitMethod(doc, chunkMarkdownSplitMethod)
		}
		return docs, nil
	case model.MimeTypeEPUB:
		docs, err := epubparser.NewEPUBParser(&epubparser.Config{
			OutputFormat: epubparser.OutputFormatMarkdown,
			ToPages:      false,
		}).Parse(ctx, r, opts...)
		if err != nil {
			return nil, err
		}
		for _, doc := range docs {
			attachChunkSplitMethod(doc, chunkMarkdownSplitMethod)
		}
		return docs, nil
	}

	// text parser fallback
	return textParser.Parse(ctx, r, opts...)
}
