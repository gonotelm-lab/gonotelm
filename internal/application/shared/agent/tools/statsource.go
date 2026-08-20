package tools

import (
	"context"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/agentize"
	pkstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

var statSourceToolParams *schema.ParamsOneOf

const StatSourceToolName = "StatSource"

func init() {
	var err error
	statSourceToolParams, err = utils.GoStruct2ParamsOneOf[StatSourceToolInput]()
	if err != nil {
		panic(err)
	}
}

type StatSourceTool struct {
	service       *agentize.Service
	sourceChecker SourcePermissionChecker
}

func NewStatSourceTool(
	s *agentize.Service,
	sourceChecker SourcePermissionChecker,
) *StatSourceTool {
	return &StatSourceTool{
		service:       s,
		sourceChecker: sourceChecker,
	}
}

var _ tool.InvokableTool = &StatSourceTool{}

type StatSourceToolInput struct {
	SourceId string `json:"source_id" jsonschema:"title=source unique identifier(32 characters uuid format),description=The input source id to get stats from."`
}

func (i *StatSourceToolInput) Normalize() (valobj.Id, error) {
	sourceID, err := valobj.NewIdFromString(i.SourceId)
	if err != nil {
		return valobj.Id{}, fmt.Errorf("source id is not valid uuid: %w", err)
	}

	return sourceID, nil
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
	args string,
	opts ...tool.Option,
) (string, error) {
	var input StatSourceToolInput
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

	result, err := s.service.StatSource(ctx, sourceID)
	if err != nil {
		return "", fmt.Errorf("stat source failed: %w", err)
	}

	output := fmt.Sprintf(`{"bytes_count":%d,"runes_count":%d,"lines_count":%d,"abstract":"%s"}`,
		result.Bytes, result.Runes, result.Lines, result.Abstract)

	return output, nil
}
