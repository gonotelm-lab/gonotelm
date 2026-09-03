package pptx

import (
	"context"
	"fmt"
	"io"

	einoparser "github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/eino-ext/util"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

const markdownContentType = "text/markdown"

// Config 配置 PptxParser。当前为空，为后续选项（如是否渲染备注）预留。
type Config struct{}

// Parser 将 pptx 演示文稿转换为 markdown，只保留有语义的内容：
// 标题、文本段落与列表、表格、演讲者备注、图片 alt text；
// 丢弃主题、版式、动画与图片字节等纯表现层内容。
type Parser struct{}

var _ einoparser.Parser = (*Parser)(nil)

// NewParser 创建解析器，cfg 可以为 nil。
func NewParser(config *Config) *Parser {
	return &Parser{}
}

func (p *Parser) Parse(
	ctx context.Context,
	reader io.Reader,
	opts ...einoparser.Option,
) ([]*schema.Document, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("pptx parser read all from reader failed: %w", err)
	}

	pres, err := parsePresentation(ctx, data)
	if err != nil {
		return nil, err
	}

	content, err := renderPresentationMarkdown(ctx, pres)
	if err != nil {
		return nil, err
	}

	commonOpts := einoparser.GetCommonOptions(nil, opts...)
	baseMeta := util.CopyMeta(commonOpts.ExtraMeta)
	baseMeta["content_type"] = markdownContentType
	if commonOpts.URI != "" {
		baseMeta[einoparser.MetaKeySource] = commonOpts.URI
	}

	return []*schema.Document{
		{
			ID:       uuid.NewV4().String(),
			Content:  content,
			MetaData: baseMeta,
		},
	}, nil
}
