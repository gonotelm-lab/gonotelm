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

var bashToolParams *schema.ParamsOneOf

const BashToolName = "Bash"

// 命令最大超时时间 3 分钟
var maxBashTimeout = (3 * time.Minute).Milliseconds()

func init() {
	var err error
	bashToolParams, err = utils.GoStruct2ParamsOneOf[BashToolInput]()
	if err != nil {
		panic(err)
	}
}

// BashTool 在沙箱中执行命令
type BashTool struct {
	sandbox sandboxentity.Sandbox
}

func NewBashTool(sb sandboxentity.Sandbox) *BashTool {
	return &BashTool{
		sandbox: sb,
	}
}

var _ tool.InvokableTool = &BashTool{}

type BashToolInput struct {
	Command string `json:"command"            jsonschema:"title=command to execute,description=The command to execute"`
	Timeout int    `json:"timeout,omitempty"  jsonschema_description:"Optional timeout in milliseconds (max 180000)"`
}

// 命令输出最大返回字符数
const maxBashOutputChars = 30000

func (t *BashTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: BashToolName,
		Desc: "Execute a shell command inside the sandbox and return its exit code, stdout and stderr.\n\n" +
			"Guidelines:\n" +
			"- Quote file paths containing spaces with double quotes.\n" +
			"- Combine multiple commands with ';' or '&&' instead of newlines. Prefer workspace-relative or absolute paths over cd.\n" +
			"- timeout is optional, in milliseconds (max 180000ms / 3 minutes). If not specified, defaults to 3 minutes.\n" +
			"- If the output exceeds 30000 characters, it will be truncated.",
		ParamsOneOf: bashToolParams,
	}, nil
}

func (t *BashTool) InvokableRun(
	ctx context.Context,
	args string,
	opts ...tool.Option,
) (string, error) {
	var input BashToolInput
	err := sonic.Unmarshal(pkstring.AsBytes(args), &input)
	if err != nil {
		return "", fmt.Errorf("args input is not valid json: %w", err)
	}

	if input.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	timeoutMs := int64(input.Timeout)
	if timeoutMs <= 0 {
		timeoutMs = maxBashTimeout
	} else if timeoutMs > maxBashTimeout {
		timeoutMs = maxBashTimeout
	}
	timeout := time.Duration(timeoutMs) * time.Millisecond

	exec, err := t.sandbox.Run(ctx, sandboxentity.Command{
		Command: input.Command,
		Timeout: timeout,
	})
	if err != nil {
		return "", fmt.Errorf("run command failed: %w", err)
	}

	var builder strings.Builder
	builder.Grow(512)
	fmt.Fprintf(&builder, "exit_code: %d\n", exec.ExitCode)
	if len(exec.Stdout) > 0 {
		fmt.Fprintf(&builder, "<stdout>\n%s\n</stdout>", exec.Stdout)
	}
	if len(exec.Stderr) > 0 {
		fmt.Fprintf(&builder, "\n<stderr>\n%s\n</stderr>", exec.Stderr)
	}

	output := builder.String()
	if len(output) > maxBashOutputChars {
		output = output[:maxBashOutputChars] + "\n... [output truncated]"
	}

	return output, nil
}
