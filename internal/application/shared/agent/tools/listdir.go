package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	sandboxentity "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	pkstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

var listDirToolParams *schema.ParamsOneOf

const ListDirToolName = "ListDir"

func init() {
	var err error
	listDirToolParams, err = utils.GoStruct2ParamsOneOf[ListDirToolInput]()
	if err != nil {
		panic(err)
	}
}

// ListDirTool 列出沙箱中目录的内容
type ListDirTool struct {
	sandbox sandboxentity.Sandbox
}

func NewListDirTool(sb sandboxentity.Sandbox) *ListDirTool {
	return &ListDirTool{
		sandbox: sb,
	}
}

var _ tool.InvokableTool = &ListDirTool{}

type ListDirToolInput struct {
	Path string `json:"path" jsonschema:"title=directory path in sandbox,description=The absolute path of the directory to list"`
}

func (t *ListDirTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: ListDirToolName,
		Desc: "List the contents of a directory inside the sandbox by its absolute path. " +
			"Each entry is returned in one line in format 'TYPE\\tSIZE\\tMODIFIED_AT\\tPATH', " +
			"where TYPE is 'dir' or 'file'.",
		ParamsOneOf: listDirToolParams,
	}, nil
}

func (t *ListDirTool) InvokableRun(
	ctx context.Context,
	args string,
	opts ...tool.Option,
) (string, error) {
	var input ListDirToolInput
	err := sonic.Unmarshal(pkstring.AsBytes(args), &input)
	if err != nil {
		return "", fmt.Errorf("args input is not valid json: %w", err)
	}

	if input.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	items, err := t.sandbox.ListDir(ctx, input.Path)
	if err != nil {
		return "", fmt.Errorf("list directory %s failed: %w", input.Path, err)
	}

	var builder strings.Builder
	builder.Grow(len(items) * 64)
	for _, item := range items {
		fmt.Fprintf(&builder, "%s\t%d\t%s\t%s\n",
			item.Type, item.Size, item.ModifiedAt.Format(time.RFC3339), item.Path)
	}

	return builder.String(), nil
}
