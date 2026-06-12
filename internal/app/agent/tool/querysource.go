package tool

import (
	"context"
	"fmt"

	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	pkstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/string/markdown"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

var querySourceToolParams *schema.ParamsOneOf

const QuerySourceToolName = "query_source"

func init() {
	var err error
	querySourceToolParams, err = utils.GoStruct2ParamsOneOf[QuerySourceToolInput]()
	if err != nil {
		panic(err)
	}
}

const (
	defaultQuerySourceToolCount = 10
	maxQuerySourceToolCount     = 50
)

// 在指定来源中进行相似性搜索 返回相似度较高的文档片段
type QuerySourceTool struct {
	biz        *bizsource.AgentBiz
	checker    SourceChecker
	notebookId uuid.UUID
}

func NewQuerySourceTool(
	biz *bizsource.AgentBiz,
	notebookId uuid.UUID,
	checker SourceChecker,
) *QuerySourceTool {
	return &QuerySourceTool{
		biz:        biz,
		checker:    checker,
		notebookId: notebookId,
	}
}

var _ tool.InvokableTool = &QuerySourceTool{}

type QuerySourceToolInput struct {
	SourceIds []string `json:"source_ids"      jsonschema:"title=source unique identifier(32 characters uuid format),description=The input source ids to query from."`
	Query     string   `json:"query"           jsonschema:"description=The query to search in the source."`
	Count     int      `json:"count,omitempty" jsonschema:"description=The number of documents to return. (defaults to 10. max is 50)"`
}

func (i *QuerySourceToolInput) Normalize() ([]uuid.UUID, error) {
	if i.Count <= 0 {
		i.Count = defaultQuerySourceToolCount
	}

	if i.Count > maxQuerySourceToolCount {
		i.Count = maxQuerySourceToolCount
	}

	if i.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	sourceIDs := make([]uuid.UUID, 0, len(i.SourceIds))
	for _, sourceId := range i.SourceIds {
		sourceID, err := uuid.ParseString(sourceId)
		if err != nil {
			return nil, fmt.Errorf("source id is not valid uuid: %w", err)
		}

		sourceIDs = append(sourceIDs, sourceID)
	}

	return sourceIDs, nil
}

var querySourceToolTableHeader = []string{"source_id", "doc_id", "content", "score"}

func (s *QuerySourceTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: QuerySourceToolName,
		Desc: "Query the source by given source id and query text. " +
			"Perform similarity search in the given source with multiple splitted document chunks. " +
			"Output will be a list of most-matched document chunks from the source. " +
			"If matches found, output format is a markdown table with columns: 'source_id', 'doc_id', 'content', 'score'. " +
			"Otherwise, output will be a string '(no matches found)'.",
		ParamsOneOf: querySourceToolParams,
	}, nil
}

func (s *QuerySourceTool) InvokableRun(
	ctx context.Context,
	args string, // json format
	opts ...tool.Option,
) (string, error) {
	var input QuerySourceToolInput
	err := sonic.Unmarshal(pkstring.AsBytes(args), &input)
	if err != nil {
		return "", fmt.Errorf("args input is not valid json: %w", err)
	}

	sourceIds, err := input.Normalize()
	if err != nil {
		return "", fmt.Errorf("source id is not valid uuid: %w", err)
	}

	if s.checker != nil {
		for _, sourceId := range sourceIds {
			if err := s.checker.CheckPermission(ctx, sourceId); err != nil {
				return "", permissionDeniedForSource(sourceId)
			}
		}
	}

	matches, err := s.biz.SearchSource(ctx,
		&bizsource.AgentSearchSourceQuery{
			NotebookId: s.notebookId,
			SourceIds:  sourceIds,
			Target:     input.Query,
			Count:      input.Count,
		})
	if err != nil {
		return "", fmt.Errorf("search source failed: %w", err)
	}

	if len(matches.Chunks) == 0 {
		return "(no matches found)", nil
	}

	builder := markdown.NewTableBuilder(querySourceToolTableHeader)

	for _, match := range matches.Chunks {
		builder.AddRow([]string{
			match.SourceId.String(),
			match.Id,
			match.Content,
			fmt.Sprintf("%.3f", match.Score),
		})
	}

	return builder.Build(), nil
}
