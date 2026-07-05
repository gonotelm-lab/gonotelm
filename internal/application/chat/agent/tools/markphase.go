package tools

import (
	"context"
	"fmt"

	pkstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

var markPhaseToolParams *schema.ParamsOneOf

const MarkPhaseToolName = "MarkPhase"

func init() {
	var err error
	markPhaseToolParams, err = utils.GoStruct2ParamsOneOf[MarkPhaseToolInput]()
	if err != nil {
		panic(err)
	}
}

func NewMarkPhaseTool() *MarkPhaseTool {
	return &MarkPhaseTool{}
}

type MarkPhaseToolInput struct {
	Summary     string `json:"summary"     jsonschema:"title=summary,description=The summary of the phase in a single sentence."`
	Description string `json:"description" jsonschema:"title=description,description=The description of the phase, describe what you are going to do in 50-100 words."`
}

type MarkPhaseTool struct{}

func (s *MarkPhaseTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: MarkPhaseToolName,
		Desc: "The task you are assigned to requires you to break down the task into phases and mark each phase. " +
			"You should mark the phase by given summary and description every time you enter a new processing phase.",
		ParamsOneOf: markPhaseToolParams,
	}, nil
}

func (s *MarkPhaseTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	var input MarkPhaseToolInput
	err := sonic.Unmarshal(pkstring.AsBytes(args), &input)
	if err != nil {
		return "", fmt.Errorf("args input is not valid json: %w", err)
	}

	return OkToolCallResult, nil
}
