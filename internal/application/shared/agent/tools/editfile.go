package tools

import (
	"context"
	"fmt"

	sandboxentity "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	pkstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

var editFileToolParams *schema.ParamsOneOf

const EditFileToolName = "EditFile"

func init() {
	var err error
	editFileToolParams, err = utils.GoStruct2ParamsOneOf[EditFileToolInput]()
	if err != nil {
		panic(err)
	}
}

// EditFileTool 编辑沙箱中的文件（字符串替换）
type EditFileTool struct {
	sandbox sandboxentity.Sandbox
}

func NewEditFileTool(sb sandboxentity.Sandbox) *EditFileTool {
	return &EditFileTool{
		sandbox: sb,
	}
}

var _ tool.InvokableTool = &EditFileTool{}

type EditFileToolInput struct {
	Path      string `json:"path" jsonschema:"title=file path in sandbox,description=The absolute path of the file to edit"`
	OldString string `json:"old_string"  jsonschema:"title=old_string content,description=The exact existing content to be replaced, must match the file content exactly"`
	NewString string `json:"new_string"  jsonschema:"title=new_string content,description=The new content to replace the old content with"`
}

func (t *EditFileTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: EditFileToolName,
		Desc: "Performs exact string replacements in a file inside the sandbox.\n\n" +
			"Usage:\n" +
			"- You must at use your `ReadFile` tool at least once in the conversation before editing.\n" +
			"- When providing 'old_string', preserve the exact indentation (tabs/spaces) and whitespace as it appears in the file." +
			"- The edit fails if 'old_string' does not exist in the file.\n",
		ParamsOneOf: editFileToolParams,
	}, nil
}

func (t *EditFileTool) InvokableRun(
	ctx context.Context,
	args string,
	opts ...tool.Option,
) (string, error) {
	var input EditFileToolInput
	err := sonic.Unmarshal(pkstring.AsBytes(args), &input)
	if err != nil {
		return "", fmt.Errorf("args input is not valid json: %w", err)
	}

	if input.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	if err := t.sandbox.EditFile(ctx, input.Path, input.OldString, input.NewString); err != nil {
		return "", fmt.Errorf("edit file failed: %w", err)
	}

	return OkToolCallResult, nil
}
