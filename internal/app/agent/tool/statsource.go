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

var statSourceToolParams *schema.ParamsOneOf

const StatSourceToolName = "stat_source"

func init() {
	var err error
	statSourceToolParams, err = utils.GoStruct2ParamsOneOf[StatSourceToolInput]()
	if err != nil {
		panic(err)
	}
}

type StatSourceTool struct {
	biz *bizsource.BizForAgent
	checker SourceChecker
}

func NewStatSourceTool(biz *bizsource.BizForAgent, checker SourceChecker) *StatSourceTool {
	return &StatSourceTool{biz: biz, checker: checker}
}

var _ tool.InvokableTool = &StatSourceTool{}

type StatSourceToolInput struct {
	SourceId string `json:"source_id" jsonschema:"title=source unique identifier(32 characters uuid format),description=The input source id to get stats from."`
}

func (s *StatSourceTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: StatSourceToolName,
		Desc: "Stat the content of a source by given source id. " +
			"Output includes bytes count, runes count, lines count, and source abstract (summary of the source content).",
		ParamsOneOf: statSourceToolParams,
	}, nil
}

func (s *StatSourceTool) InvokableRun(
	ctx context.Context,
	args string, // json format
	opts ...tool.Option,
) (string, error) {
	var input StatSourceToolInput
	err := sonic.Unmarshal(pkstring.AsBytes(args), &input)
	if err != nil {
		return "", fmt.Errorf("args input is not valid json: %w", err)
	}

	sourceID, err := uuid.ParseString(input.SourceId)
	if err != nil {
		return "", fmt.Errorf("source id is not valid uuid: %w", err)
	}

	if s.checker != nil {
		if err := s.checker.CheckPermission(ctx, sourceID); err != nil {
			return "", permissionDeniedForSource(sourceID)
		}
	}

	result, err := s.biz.StatSource(ctx, sourceID)
	if err != nil {
		return "", fmt.Errorf("stat source failed: %w", err)
	}

	output := fmt.Sprintf(`{"bytes_count":%d,"runes_count":%d,"lines_count":%d,"abstract":"%s"}`,
		result.Bytes, result.Runes, result.Lines, result.Abstract)

	return output, nil
}
