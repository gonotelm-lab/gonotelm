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

var readFileToolParams *schema.ParamsOneOf

const ReadFileToolName = "ReadFile"

// 读取沙箱文件时单行最大返回字符数
const maxReadFileLineChars = 2000

func init() {
	var err error
	readFileToolParams, err = utils.GoStruct2ParamsOneOf[ReadFileToolInput]()
	if err != nil {
		panic(err)
	}
}

// ReadFileTool 按行读取沙箱中的文件内容
type ReadFileTool struct {
	sandbox sandboxentity.Sandbox
}

func NewReadFileTool(sb sandboxentity.Sandbox) *ReadFileTool {
	return &ReadFileTool{
		sandbox: sb,
	}
}

var _ tool.InvokableTool = &ReadFileTool{}

type ReadFileToolInput struct {
	FilePath string `json:"file_path"        jsonschema:"title=file path in sandbox,description=The absolute path to the file to read"`
	Offset   int    `json:"offset,omitempty" jsonschema_description:"1-based line number to start reading from. Only provide if the file is too large to read at once. If omitted or 0, reads from line 1."`
	Limit    int    `json:"limit,omitempty"  jsonschema_description:"The number of lines to read. Only provide if the file is too large to read at once. If omitted or 0, reads to the end."`
}

func (t *ReadFileTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: ReadFileToolName,
		Desc: "Read a file inside the sandbox by its absolute path.\n\n" +
			"Usage:\n" +
			"- file_path must be an absolute path\n" +
			"- By default reads the whole file from the beginning; use offset (1-based line number) " +
			"and limit to read long files in chunks\n" +
			"- Lines longer than 2000 characters will be truncated\n" +
			"- Results are returned in cat -n format 'LINE_NUMBER|LINE_CONTENT', line numbers start at 1",
		ParamsOneOf: readFileToolParams,
	}, nil
}

func (t *ReadFileTool) InvokableRun(
	ctx context.Context,
	args string,
	opts ...tool.Option,
) (string, error) {
	var input ReadFileToolInput
	err := sonic.Unmarshal(pkstring.AsBytes(args), &input)
	if err != nil {
		return "", fmt.Errorf("args input is not valid json: %w", err)
	}

	if input.FilePath == "" {
		return "", fmt.Errorf("file_path is required")
	}

	data, err := t.sandbox.ReadFile(ctx, input.FilePath)
	if err != nil {
		return "", fmt.Errorf("read file %s failed: %w", input.FilePath, err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1] // 去掉结尾换行产生的空行
	}

	start := max(1, input.Offset) - 1
	if start >= len(lines) {
		return "", nil
	}
	end := len(lines)
	if input.Limit > 0 && start+input.Limit < end {
		end = start + input.Limit
	}

	var builder strings.Builder
	builder.Grow(512)
	for i := start; i < end; i++ {
		line := lines[i]
		if len(line) > maxReadFileLineChars {
			line = line[:maxReadFileLineChars] + "... [line truncated]"
		}
		fmt.Fprintf(&builder, "%d|%s\n", i+1, line)
	}

	return builder.String(), nil
}
