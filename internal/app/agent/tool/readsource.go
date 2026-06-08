package tool

import (
	"context"
	"fmt"
	"strings"

	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	pkstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

var readSourceToolParams *schema.ParamsOneOf

const ReadSourceToolName = "read_source"

func init() {
	var err error
	readSourceToolParams, err = utils.GoStruct2ParamsOneOf[ReadSourceToolInput]()
	if err != nil {
		panic(err)
	}
}

// 读取来源的内容
//
// 来源是解析后的内容
type ReadSourceTool struct {
	biz     *bizsource.BizForAgent
	checker SourceChecker
}

func NewReadSourceTool(biz *bizsource.BizForAgent, checker SourceChecker) *ReadSourceTool {
	return &ReadSourceTool{biz: biz, checker: checker}
}

var _ tool.InvokableTool = &ReadSourceTool{}

type ReadSourceToolInput struct {
	SourceId  string `json:"source_id"            jsonschema:"title=source unique identifier(32 characters uuid format),description=The input source id to read from"`
	StartLine int    `json:"start_line,omitempty" jsonschema_description:"1-based start line number. If omitted or 0, reading starts from line 1."`
	LineCount int    `json:"line_count,omitempty" jsonschema_description:"Maximum number of lines to read from start_line. If omitted or 0, reads to the end."`
}

func (s *ReadSourceTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: ReadSourceToolName,
		Desc: "Read the content of a source by given source id. Output always include line number " +
			"in format 'LINE_NUMBER|LINE_CONTENT' (1-indexed). Supports reading partial content " +
			"by specifying start_line and line_count for large contents.",
		ParamsOneOf: readSourceToolParams,
	}, nil
}

func (s *ReadSourceTool) InvokableRun(
	ctx context.Context,
	args string, // json format
	opts ...tool.Option,
) (string, error) {
	var input ReadSourceToolInput
	err := sonic.Unmarshal(pkstring.AsBytes(args), &input)
	if err != nil {
		return "", fmt.Errorf("args input is not valid json: %w", err)
	}

	sourceId, err := uuid.ParseString(input.SourceId)
	if err != nil {
		return "", fmt.Errorf("source id is not valid uuid: %w", err)
	}

	if s.checker != nil {
		if err := s.checker.CheckPermission(ctx, sourceId); err != nil {
			return "", permissionDeniedForSource(sourceId)
		}
	}

	result, err := s.biz.ReadSource(ctx,
		&bizsource.AgentReadSourceQuery{
			SourceId: sourceId,
			Offset:   max(0, input.StartLine),
			Limit:    max(0, input.LineCount),
		})
	if err != nil {
		return "", fmt.Errorf("read source failed: %w", err)
	}

	var builder strings.Builder
	builder.Grow(512)
	for _, line := range result.Lines {
		fmt.Fprintf(&builder, "%d|%s\n", line.LineNo, line.Line)
	}

	return builder.String(), nil
}
