package tools

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	agentv2 "github.com/gonotelm-lab/gonotelm/pkg/agent/v2"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	pkstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"golang.org/x/sync/semaphore"
)

var subagentToolParams *schema.ParamsOneOf

const SubagentToolName = "Subagent"

const (
	defaultSubagentMaxRound    = 15
	defaultSubagentConcurrency = 3
	defaultSubagentResultChars = 20000

	subagentResultFooter        = "\n</subagent_result>"
	subagentResultTruncatedMark = "\n... [subagent result truncated]"

	defaultSubagentSystemPrompt = "You are a subagent executing a self-contained task in an isolated context window. " +
		"You cannot see the calling agent's conversation and you cannot ask questions. " +
		"Use the tools available to you, then finish with a concise, self-contained result that fully answers the task."
)

func init() {
	var err error
	subagentToolParams, err = utils.GoStruct2ParamsOneOf[SubagentToolInput]()
	if err != nil {
		panic(err)
	}
}

type SubagentConfig struct {
	BaseLLM      model.ToolCallingChatModel
	Options      []model.Option
	SystemPrompt string
	Tools        map[string]tool.InvokableTool // 绑定到subagent上的工具

	MaxRound       int
	Timeout        time.Duration
	MaxConcurrency int
	MaxCallsPerRun int
	MaxResultChars int

	Verbose bool
}

type SubagentTool struct {
	runner       *agentv2.StatelessRunner
	systemPrompt string

	timeout        time.Duration
	maxCallsPerRun int
	maxResultChars int

	sem   *semaphore.Weighted
	calls atomic.Int64
}

func NewSubagentTool(ctx context.Context, cfg SubagentConfig) (*SubagentTool, error) {
	if cfg.BaseLLM == nil {
		return nil, fmt.Errorf("base llm is required")
	}

	maxRound := cfg.MaxRound
	if maxRound <= 0 {
		maxRound = defaultSubagentMaxRound
	}

	maxConcurrency := cfg.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = defaultSubagentConcurrency
	}

	maxResultChars := cfg.MaxResultChars
	if maxResultChars <= 0 {
		maxResultChars = defaultSubagentResultChars
	}

	systemPrompt := strings.TrimSpace(cfg.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = defaultSubagentSystemPrompt
	}

	toolSet, err := resolveSubagentTools(cfg.Tools)
	if err != nil {
		return nil, err
	}

	runner, err := agentv2.NewStatelessRunner(ctx, agentv2.RunnerConfig{
		Name:     "subagent",
		Model:    cfg.BaseLLM,
		Options:  cfg.Options,
		Tools:    toolSet,
		MaxRound: maxRound,
		Verbose:  cfg.Verbose,
	})
	if err != nil {
		return nil, fmt.Errorf("new subagent runner: %w", err)
	}

	return &SubagentTool{
		runner:         runner,
		systemPrompt:   systemPrompt,
		timeout:        cfg.Timeout,
		maxCallsPerRun: cfg.MaxCallsPerRun,
		maxResultChars: maxResultChars,
		sem:            semaphore.NewWeighted(int64(maxConcurrency)),
	}, nil
}

var _ tool.InvokableTool = &SubagentTool{}

type SubagentToolInput struct {
	Name   string `json:"name"   jsonschema:"title=subagent name,description=A short name the model gives to this subagent, used to identify it."`
	Title  string `json:"title"  jsonschema:"title=subagent title,description=One sentence describing what this subagent is doing."`
	Prompt string `json:"prompt" jsonschema:"title=task for the subagent,description=The self-contained task the subagent must execute. It cannot see this conversation so include the goal the relevant context constraints and the expected output format."`
}

func (t *SubagentTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: SubagentToolName,
		Desc: "Spawn a subagent that executes a self-contained task in its own isolated context window. " +
			"Only the subagent's final result is returned to you; its intermediate steps and full transcript stay out of this conversation.\n\n" +
			"The subagent cannot see this conversation so `prompt` must be fully self-contained: " +
			"include the goal the relevant context constraints and the expected output format.\n" +
			"Give the subagent a short `name` to identify it and a one-sentence `title`.\n\n" +
			"Guidelines:\n" +
			"- Use it to keep your own context small: delegate broad exploration or long multi-step work.\n" +
			"- To run independent tasks in parallel issue multiple Subagent calls in the same turn.\n" +
			"- Do NOT use it for quick lookups you can do yourself in one or two tool calls.",
		ParamsOneOf: subagentToolParams,
	}, nil
}

func (t *SubagentTool) InvokableRun(
	ctx context.Context,
	args string,
	opts ...tool.Option,
) (string, error) {
	var input SubagentToolInput
	err := sonic.Unmarshal(pkstring.AsBytes(args), &input)
	if err != nil {
		return "", fmt.Errorf("args input is not valid json: %w", err)
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		return "", fmt.Errorf("title is required")
	}

	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	if t.maxCallsPerRun > 0 {
		if n := t.calls.Add(1); n > int64(t.maxCallsPerRun) {
			return "", fmt.Errorf("subagent call limit exceeded: max %d call(s) per run", t.maxCallsPerRun)
		}
	}

	if err := t.sem.Acquire(ctx, 1); err != nil {
		return "", fmt.Errorf("acquire subagent slot failed: %w", err)
	}
	defer t.sem.Release(1)

	content, err := t.run(ctx, name, title, prompt)
	if err != nil {
		return "", err
	}

	return formatSubagentResult(name, title, content, t.maxResultChars), nil
}

// run 在独立的 Session 中执行子代理任务，共享不可变 Runner。
func (t *SubagentTool) run(ctx context.Context, name, title, prompt string) (string, error) {
	if t.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.timeout)
		defer cancel()
	}

	ctx = pkgcontext.WithSubagent(ctx)

	session := agentv2.NewStatelessSession()
	session.System = buildSubagentSystemPrompt(t.systemPrompt, name, title)
	session.Append(schema.UserMessage(prompt))

	res, err := t.runner.Run(ctx, session, agentv2.StatelessRunConfig{})
	if err != nil {
		return "", fmt.Errorf("subagent %s run failed: %w", name, err)
	}
	if res.Final == nil {
		return "", fmt.Errorf("subagent %s returned empty final message", name)
	}

	return res.Final.Content, nil
}

func resolveSubagentTools(declared map[string]tool.InvokableTool) ([]agentv2.Tool, error) {
	resolved := make([]agentv2.Tool, 0, len(declared))
	for name, invokable := range declared {
		if invokable == nil {
			return nil, fmt.Errorf("subagent tool %s is nil", name)
		}

		if name == SubagentToolName {
			return nil, fmt.Errorf("subagent cannot bind %s: subagent recursion is disabled", SubagentToolName)
		}

		info, err := invokable.Info(context.Background())
		if err != nil {
			return nil, fmt.Errorf("get subagent tool %s info failed: %w", name, err)
		}
		if info == nil {
			return nil, fmt.Errorf("subagent tool %s info is nil", name)
		}
		if info.Name != name {
			return nil, fmt.Errorf("subagent tool map key %s does not match tool info name %s", name, info.Name)
		}

		resolved = append(resolved, invokable)
	}

	return resolved, nil
}

func buildSubagentSystemPrompt(systemPrompt, name, title string) string {
	var builder strings.Builder
	builder.Grow(len(systemPrompt) + len(name) + len(title) + 32)
	builder.WriteString(strings.TrimSpace(systemPrompt))
	builder.WriteString("\n\nYour name: ")
	builder.WriteString(name)
	builder.WriteString("\nYour task: ")
	builder.WriteString(title)
	return builder.String()
}

func formatSubagentResult(name, title, content string, maxChars int) string {
	header := fmt.Sprintf("<subagent_result name=%q title=%q>\n", name, title)

	body := content
	if maxChars > 0 {
		// 只截断正文，包装标签必须完整闭合。
		budget := maxChars - utf8.RuneCountInString(header) - utf8.RuneCountInString(subagentResultFooter)
		if budget <= 0 {
			body = ""
		} else if runes := []rune(content); len(runes) > budget {
			keep := max(0, budget-utf8.RuneCountInString(subagentResultTruncatedMark))
			body = string(runes[:keep]) + subagentResultTruncatedMark
		}
	}

	return header + body + subagentResultFooter
}
