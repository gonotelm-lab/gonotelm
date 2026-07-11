package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/agentize"
	pkstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

var readSourceToolParams *schema.ParamsOneOf

const ReadSourceToolName = "ReadSource"

func init() {
	var err error
	readSourceToolParams, err = utils.GoStruct2ParamsOneOf[ReadSourceToolInput]()
	if err != nil {
		panic(err)
	}
}

// ReadSourceTool 读取来源的内容（解析后的内容）
type ReadSourceTool struct {
	service       *agentize.Service
	sourceChecker SourcePermissionChecker
}

func NewReadSourceTool(
	s *agentize.Service,
	sourceChecker SourcePermissionChecker,
) *ReadSourceTool {
	return &ReadSourceTool{
		service:       s,
		sourceChecker: sourceChecker,
	}
}

var _ tool.InvokableTool = &ReadSourceTool{}

type ReadSourceToolInput struct {
	SourceId  string `json:"source_id"            jsonschema:"title=source unique identifier(32 characters uuid format),description=The input source id to read from"`
	StartLine int    `json:"start_line,omitempty" jsonschema_description:"1-based start line number. If omitted or 0, reading starts from line 1."`
	LineCount int    `json:"line_count,omitempty" jsonschema_description:"Maximum number of lines to read from start_line. If omitted or 0, reads to the end."`
}

func (i *ReadSourceToolInput) Normalize() (valobj.Id, error) {
	sourceID, err := valobj.NewIdFromString(i.SourceId)
	if err != nil {
		return valobj.Id{}, fmt.Errorf("source id is not valid uuid: %w", err)
	}

	return sourceID, nil
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
	args string,
	opts ...tool.Option,
) (string, error) {
	var input ReadSourceToolInput
	err := sonic.Unmarshal(pkstring.AsBytes(args), &input)
	if err != nil {
		return "", fmt.Errorf("args input is not valid json: %w", err)
	}

	sourceID, err := input.Normalize()
	if err != nil {
		return "", err
	}

	if err := s.sourceChecker.Check(ctx, []valobj.Id{sourceID}); err != nil {
		return "", err
	}

	result, err := s.service.ReadSource(ctx, &agentize.ReadSourceQuery{
		SourceId: sourceID,
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
