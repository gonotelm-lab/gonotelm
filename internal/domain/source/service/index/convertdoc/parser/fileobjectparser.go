package parser

import (
	"context"
	"io"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"

	docxparser "github.com/cloudwego/eino-ext/components/document/parser/docx"
	einoparser "github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	epubparser "github.com/gonotelm-lab/gonotelm/pkg/eino-ext/parser/epub"
	pdfparser "github.com/gonotelm-lab/gonotelm/pkg/eino-ext/parser/pdf"
	pptxparser "github.com/gonotelm-lab/gonotelm/pkg/eino-ext/parser/pptx"
	xlsxparser "github.com/gonotelm-lab/gonotelm/pkg/eino-ext/parser/xlsx"
	"github.com/gonotelm-lab/gonotelm/pkg/eino-ext/util"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type FileObjectParser struct {
	imageInterpreter adapter.ImageInterpreter
}

func NewFileObjectHandler(imageInterpreter adapter.ImageInterpreter) *FileObjectParser {
	return &FileObjectParser{imageInterpreter: imageInterpreter}
}

var _ einoparser.Parser = (*FileObjectParser)(nil)

func (p *FileObjectParser) Parse(
	ctx context.Context,
	objectReader io.Reader,
	opt ...einoparser.Option,
) ([]*schema.Document, error) {
	customOpts := &customParseOption{}
	einoparser.GetImplSpecificOptions(customOpts, opt...)

	return p.parseByMime(
		ctx,
		objectReader,
		ResolveSourceMime(customOpts.fileMime, customOpts.fileExt),
		opt...,
	)
}

func (p *FileObjectParser) parseByMime(
	ctx context.Context,
	objectReader io.Reader,
	mimeType string,
	opts ...einoparser.Option,
) ([]*schema.Document, error) {
	textParser := einoparser.TextParser{}

	switch mimeType {
	case entity.MimeTypeText, entity.MimeTypeMarkdown, entity.MimeTypeCSV: // plain text / markdown / csv
		return textParser.Parse(ctx, objectReader, opts...)
	case entity.MimeTypePDF:
		// output will try to be markdown
		return doParser(ctx, objectReader, pdfparser.NewPDFParser(nil), opts...)
	case entity.MimeTypeWord:
		wordParser, _ := docxparser.NewDocxParser(ctx, &docxparser.Config{IncludeTables: true})
		return doParser(ctx, objectReader, wordParser, opts...)
	case entity.MimeTypeXLSX:
		return doParser(ctx, objectReader, xlsxparser.NewXLSXParser(nil), opts...)
	case entity.MimeTypePPTX:
		// 只保留语义内容（标题/列表/表格/备注/图片alt）
		return doParser(ctx, objectReader, pptxparser.NewParser(nil), opts...)
	case entity.MimeTypeEPUB:
		epubParser := epubparser.NewEPUBParser(&epubparser.Config{
			OutputFormat: epubparser.OutputFormatMarkdown,
			ToPages:      false,
		})
		return doParser(ctx, objectReader, epubParser, opts...)
	case entity.MimeTypeJPEG, entity.MimeTypePNG, entity.MimeTypeWebP:
		// parse image as text (image interpreter)
		imageParser := &imageFileObjectParser{
			interpreter: p.imageInterpreter,
			mimeType:    mimeType,
		}
		return doParser(ctx, objectReader, imageParser, opts...)
	}

	// text parser fallback
	return textParser.Parse(ctx, objectReader, opts...)
}

func doParser(ctx context.Context, reader io.Reader,
	docParser einoparser.Parser, opts ...einoparser.Option,
) ([]*schema.Document, error) {
	return docParser.Parse(ctx, reader, opts...)
}

type imageFileObjectParser struct {
	mimeType    string
	interpreter adapter.ImageInterpreter
}

func (p *imageFileObjectParser) Parse(
	ctx context.Context,
	objectReader io.Reader,
	opts ...einoparser.Option,
) ([]*schema.Document, error) {
	imageText, err := p.interpreter.InterpretReader(ctx, objectReader)
	if err != nil {
		return nil, err
	}

	commonOpts := einoparser.GetCommonOptions(nil, opts...)
	baseMeta := util.CopyMeta(commonOpts.ExtraMeta)
	baseMeta["content_type"] = p.mimeType
	if commonOpts.URI != "" {
		baseMeta[einoparser.MetaKeySource] = commonOpts.URI
	}

	return []*schema.Document{
		{
			ID:       uuid.NewV4().String(),
			Content:  imageText,
			MetaData: baseMeta,
		},
	}, nil
}
