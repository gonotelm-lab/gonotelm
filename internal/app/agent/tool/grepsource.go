package tool

import (
	"context"
	"fmt"

	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/pkg/rg"
	pkstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

var grepSourceToolParams *schema.ParamsOneOf

const GrepSourceToolName = "grep_source"

func init() {
	var err error
	grepSourceToolParams, err = utils.GoStruct2ParamsOneOf[GrepSourceToolInput]()
	if err != nil {
		panic(err)
	}
}

type GrepSourceTool struct {
	biz     *bizsource.BizForAgent
	checker SourceChecker
}

func NewGrepSourceTool(biz *bizsource.BizForAgent, checker SourceChecker) *GrepSourceTool {
	return &GrepSourceTool{biz: biz, checker: checker}
}

var _ tool.InvokableTool = &GrepSourceTool{}

type GrepSourceToolInput struct {
	SourceId string `json:"source_id" jsonschema:"title=source unique identifier(32 characters uuid format),description=The input source id to read from"`
	*rg.Params
}

func (i *GrepSourceToolInput) Normalize() (*rg.Params, uuid.UUID, error) {
	if i.Params == nil {
		i.Params = &rg.Params{}
	}

	if i.Pattern == "" {
		return nil, uuid.UUID{}, fmt.Errorf("pattern is required")
	}

	sourceID, err := uuid.ParseString(i.SourceId)
	if err != nil {
		return nil, uuid.UUID{}, err
	}

	return i.Params, sourceID, nil
}

func (s *GrepSourceTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        GrepSourceToolName,
		Desc:        "Grep the content of a source by given source id and regular expression pattern.",
		ParamsOneOf: grepSourceToolParams,
	}, nil
}

func (s *GrepSourceTool) InvokableRun(
	ctx context.Context,
	args string, // json format
	opts ...tool.Option,
) (string, error) {
	var input GrepSourceToolInput
	err := sonic.Unmarshal(pkstring.AsBytes(args), &input)
	if err != nil {
		return "", fmt.Errorf("args input is not valid json: %w", err)
	}

	params, sourceID, err := input.Normalize()
	if err != nil {
		return "", err
	}

	if s.checker != nil {
		if err := s.checker.CheckPermission(ctx, sourceID); err != nil {
			return "", permissionDeniedForSource(sourceID)
		}
	}

	content, err := s.biz.GetSourceContent(ctx, sourceID)
	if err != nil {
		return "", fmt.Errorf("get source content failed: %w", err)
	}

	output, err := rg.Grep(content, params)
	if err != nil {
		return "", fmt.Errorf("grep source content failed: %w", err)
	}

	return output, nil
}
