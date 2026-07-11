package tools

import (
	"context"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/agentize"
	"github.com/gonotelm-lab/gonotelm/pkg/rg"
	pkstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

var grepSourceToolParams *schema.ParamsOneOf

const GrepSourceToolName = "GrepSource"

func init() {
	var err error
	grepSourceToolParams, err = utils.GoStruct2ParamsOneOf[GrepSourceToolInput]()
	if err != nil {
		panic(err)
	}
}

type GrepSourceTool struct {
	service       *agentize.Service
	sourceChecker SourcePermissionChecker
}

func NewGrepSourceTool(
	s *agentize.Service,
	sourceChecker SourcePermissionChecker,
) *GrepSourceTool {
	return &GrepSourceTool{
		service:       s,
		sourceChecker: sourceChecker,
	}
}

var _ tool.InvokableTool = &GrepSourceTool{}

type GrepSourceToolInput struct {
	SourceId string `json:"source_id" jsonschema:"title=source unique identifier(32 characters uuid format),description=The input source id to read from"`
	*rg.Params
}

func (i *GrepSourceToolInput) Normalize() (*rg.Params, valobj.Id, error) {
	if i.Params == nil {
		i.Params = &rg.Params{}
	}

	if i.Pattern == "" {
		return nil, valobj.Id{}, fmt.Errorf("pattern is required")
	}

	sourceID, err := valobj.NewIdFromString(i.SourceId)
	if err != nil {
		return nil, valobj.Id{}, err
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

	if err := s.sourceChecker.Check(ctx, []valobj.Id{sourceID}); err != nil {
		return "", err
	}

	content, err := s.service.GetSourceContent(ctx, sourceID)
	if err != nil {
		return "", fmt.Errorf("get source content failed: %w", err)
	}

	output, err := rg.Grep(content, params)
	if err != nil {
		return "", fmt.Errorf("grep source content failed: %w", err)
	}

	return output, nil
}
