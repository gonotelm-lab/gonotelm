package tool

import (
	"context"
	"fmt"

	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	pkstring "github.com/gonotelm-lab/gonotelm/pkg/string"
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
	biz     *bizsource.BizForAgent
	checker SourceChecker
}

func NewQuerySourceTool(biz *bizsource.BizForAgent, checker SourceChecker) *QuerySourceTool {
	return &QuerySourceTool{biz: biz, checker: checker}
}

var _ tool.InvokableTool = &QuerySourceTool{}

type QuerySourceToolInput struct {
	SourceId string `json:"source_id"       jsonschema:"title=source unique identifier(32 characters uuid format),description=The input source id to query from."`
	Query    string `json:"query"           jsonschema:"description=The query to search in the source."`
	Count    int    `json:"count,omitempty" jsonschema:"description=The number of documents to return. (defaults to 10. max is 50)"`
}

func (i *QuerySourceToolInput) Normalize() (uuid.UUID, error) {
	if i.Count <= 0 {
		i.Count = defaultQuerySourceToolCount
	}

	if i.Count > maxQuerySourceToolCount {
		i.Count = maxQuerySourceToolCount
	}

	if i.Query == "" {
		return uuid.UUID{}, fmt.Errorf("query is required")
	}

	return uuid.ParseString(i.SourceId)
}

func (s *QuerySourceTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: QuerySourceToolName,
		Desc: "Query the source by given source id and query text. " +
			"Perform similarity search in the given source with multiple splitted chunks. " +
			"Output will be a list of most-matched chunks from the source.",
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

	sourceId, err := input.Normalize()
	if err != nil {
		return "", fmt.Errorf("source id is not valid uuid: %w", err)
	}

	if s.checker != nil {
		if err := s.checker.CheckPermission(ctx, sourceId); err != nil {
			return "", fmt.Errorf("source access denied: %w", err)
		}
	}

	matches, err := s.biz.SearchSource(ctx,
		&bizsource.AgentSearchSourceQuery{
			SourceId: sourceId,
			Target:   input.Query,
			Count:    input.Count,
		})
	if err != nil {
		return "", fmt.Errorf("search source failed: %w", err)
	}

	_ = matches

	return "", nil
}
