package tools

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync/atomic"
	"time"

	pkgagent "github.com/gonotelm-lab/gonotelm/pkg/agent"
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
	llm          model.ToolCallingChatModel
	options      []model.Option
	systemPrompt string
	tools        map[string]tool.InvokableTool

	maxRound       int
	timeout        time.Duration
	maxCallsPerRun int
	maxResultChars int
	verbose        bool

	sem   *semaphore.Weighted
	calls atomic.Int64
}

func NewSubagentTool(cfg SubagentConfig) (*SubagentTool, error) {
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

	return &SubagentTool{
		llm:            cfg.BaseLLM,
		options:        cfg.Options,
		systemPrompt:   systemPrompt,
		tools:          toolSet,
		maxRound:       maxRound,
		timeout:        cfg.Timeout,
		maxCallsPerRun: cfg.MaxCallsPerRun,
		maxResultChars: maxResultChars,
		verbose:        cfg.Verbose,
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

// run 在全新 Agent 中执行子代理任务，独立上下文、独立工具绑定
func (t *SubagentTool) run(ctx context.Context, name, title, prompt string) (string, error) {
	if t.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.timeout)
		defer cancel()
	}

	ctx = pkgcontext.WithSubagent(ctx)

	ag := pkgagent.New(pkgagent.Config[struct{}]{
		MaxRound: t.maxRound,
		BaseLLM:  t.llm,
		Options:  t.options,
		Verbose:  t.verbose,
	}, struct{}{})

	// BindTools 会持有传入的 map，而 NewFinalRoundHook 在最后一轮会 clear 它
	if err := ag.BindTools(maps.Clone(t.tools)); err != nil {
		return "", fmt.Errorf("subagent %s bind tools failed: %w", name, err)
	}

	ag.OnBeforeRound(pkgagent.NewFinalRoundHook(ag, t.maxRound))

	sysMsg := schema.SystemMessage(buildSubagentSystemPrompt(t.systemPrompt, name, title))
	ag.OnBeforeChat(func(
		ctx context.Context,
		state struct{},
		msgs []*pkgagent.EinoMessage,
	) ([]*pkgagent.EinoMessage, error) {
		newMsgs := make([]*pkgagent.EinoMessage, 0, len(msgs)+1)
		newMsgs = append(newMsgs, sysMsg)
		newMsgs = append(newMsgs, msgs...)
		return newMsgs, nil
	})

	final, err := ag.React(ctx, []*pkgagent.EinoMessage{{
		Role:    schema.User,
		Content: prompt,
	}})
	if err != nil {
		return "", fmt.Errorf("subagent %s react failed: %w", name, err)
	}

	return final.Content, nil
}

func resolveSubagentTools(declared map[string]tool.InvokableTool) (map[string]tool.InvokableTool, error) {
	resolved := make(map[string]tool.InvokableTool, len(declared))
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
		if info.Name != name {
			return nil, fmt.Errorf("subagent tool map key %s does not match tool info name %s", name, info.Name)
		}

		resolved[name] = invokable
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
	var builder strings.Builder
	builder.Grow(len(content) + 96)
	fmt.Fprintf(&builder, "<subagent_result name=%q title=%q>\n", name, title)
	builder.WriteString(content)
	builder.WriteString("\n</subagent_result>")

	result := builder.String()
	if maxChars > 0 {
		if runes := []rune(result); len(runes) > maxChars {
			result = string(runes[:maxChars]) + "\n... [subagent result truncated]"
		}
	}

	return result
}
