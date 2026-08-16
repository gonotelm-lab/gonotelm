package tools

import (
	"context"
	"fmt"
	"strings"

	sandboxentity "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	pkstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

var writeFileToolParams *schema.ParamsOneOf

const WriteFileToolName = "WriteFile"

func init() {
	var err error
	writeFileToolParams, err = utils.GoStruct2ParamsOneOf[WriteFileToolInput]()
	if err != nil {
		panic(err)
	}
}

// WriteFileTool 向沙箱中的文件写入内容
type WriteFileTool struct {
	sandbox sandboxentity.Sandbox
}

func NewWriteFileTool(sb sandboxentity.Sandbox) *WriteFileTool {
	return &WriteFileTool{
		sandbox: sb,
	}
}

var _ tool.InvokableTool = &WriteFileTool{}

type WriteFileToolInput struct {
	Path    string `json:"path"    jsonschema:"title=file path in sandbox,description=The absolute path of the file to write in the sandbox"`
	Content string `json:"content" jsonschema:"title=file content,description=The full content to write into the file, overwriting existing content"`
}

func (t *WriteFileTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: WriteFileToolName,
		Desc: "Write content into a file inside the sandbox by its absolute path. This tool will " +
			"overwrite the existing file if there is one at the provided path.\n\n" +
			"Usage:\n" +
			"- file_path must be an absolute path\n" +
			"- Content is written as-is, replacing any existing content",
		ParamsOneOf: writeFileToolParams,
	}, nil
}

func (t *WriteFileTool) InvokableRun(
	ctx context.Context,
	args string,
	opts ...tool.Option,
) (string, error) {
	var input WriteFileToolInput
	err := sonic.Unmarshal(pkstring.AsBytes(args), &input)
	if err != nil {
		return "", fmt.Errorf("args input is not valid json: %w", err)
	}

	if input.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	if err := t.sandbox.WriteFile(ctx, input.Path, strings.NewReader(input.Content)); err != nil {
		return "", fmt.Errorf("write file %s failed: %w", input.Path, err)
	}

	return OkToolCallResult, nil
}
