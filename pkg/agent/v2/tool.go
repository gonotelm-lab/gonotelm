package agent

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	oteltrace "go.opentelemetry.io/otel/trace"

	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/trace"
)

const maxVerboseRune = 200

// Tool 是 agent 可调用的工具。
type Tool = einotool.InvokableTool

// ToolPolicy 控制单轮工具的执行方式、并发、超时与结果长度。
type ToolPolicy struct {
	// Serial 为 true 时按调用顺序串行执行工具，单个工具失败不会阻塞后续调用。
	// 默认 false，即并发执行。
	Serial bool
	// MaxConcurrency 限制并发度，为 0 表示不限制；串行模式下忽略。
	MaxConcurrency int
	// Timeout 为 0 表示跟随 ctx。
	Timeout time.Duration
	// MaxResultChars 为 0 表示不截断。
	MaxResultChars int
}

type toolEntry struct {
	tool einotool.InvokableTool
	info *schema.ToolInfo
}

func validateTools(ctx context.Context, tools []Tool) ([]*toolEntry, error) {
	entries := make([]*toolEntry, 0, len(tools))
	seen := make(map[string]struct{}, len(tools))

	for idx, tool := range tools {
		if tool == nil {
			return nil, pkgerrors.ErrParams.Msgf("tool at index %d is nil", idx)
		}

		info, err := tool.Info(ctx)
		if err != nil {
			return nil, pkgerrors.Wrapf(pkgerrors.ErrParams, "get tool info at index %d failed: %v", idx, err)
		}
		if info == nil {
			return nil, pkgerrors.ErrParams.Msgf("tool at index %d has nil info", idx)
		}

		name := strings.TrimSpace(info.Name)
		if name == "" {
			return nil, pkgerrors.ErrParams.Msgf("tool at index %d has empty name", idx)
		}
		if _, ok := seen[name]; ok {
			return nil, pkgerrors.ErrParams.Msgf("duplicated tool name: %s", name)
		}
		seen[name] = struct{}{}

		entries = append(entries, &toolEntry{tool: tool, info: info})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].info.Name < entries[j].info.Name
	})

	return entries, nil
}

// executeTools 执行一轮的全部工具。返回结果按工具调用顺序排列。
// 无论并行还是串行，单个工具失败都只记录在该条结果上，不影响其余工具执行。
func (r *Runner[S]) executeTools(
	ctx context.Context,
	s *Session[S],
	calls []ToolCall,
	entries []*toolEntry,
	hooks Hooks[S],
	info RunInfo,
	policy ToolPolicy,
) ([]ToolResult, error) {
	results := make([]ToolResult, len(calls))
	toolCtxes := make([]*ToolCallContext[S], len(calls))
	for idx, call := range calls {
		results[idx] = ToolResult{
			Id:        call.Id,
			Name:      call.Name,
			Arguments: call.Arguments,
		}
		toolCtxes[idx] = &ToolCallContext[S]{
			Session: s,
			Info:    info,
			Call:    call,
		}

		if err := hooks.OnToolCallStart(ctx, toolCtxes[idx]); err != nil {
			return nil, pkgerrors.WithMessagef(err, "tool call start hook failed at %s", call.Name)
		}
	}

	var (
		hookMu  sync.Mutex
		hookErr error
	)

	runOne := func(idx int) {
		call := calls[idx]
		entry := findTool(entries, call.Name)
		start := time.Now()

		toolCtx := ctx
		if policy.Timeout > 0 {
			var cancel context.CancelFunc
			toolCtx, cancel = context.WithTimeout(ctx, policy.Timeout)
			defer cancel()
		}

		toolCtx, span := trace.GetOtelTracer().Start(
			toolCtx,
			spanAgentTool,
			oteltrace.WithAttributes(
				attrAgentName.String(r.name),
				attrAgentToolName.String(call.Name),
				attrAgentToolCallID.String(call.Id),
			),
		)
		defer span.End()

		var (
			out      string
			err      error
			panicked bool
		)
		if entry == nil {
			err = fmt.Errorf("tool %s not found", call.Name)
		} else {
			out, err, panicked = runTool(toolCtx, entry.tool, call.Name, call.Arguments)
		}

		end := time.Now()
		results[idx].Duration = end.Sub(start)
		results[idx].Error = err
		if err != nil {
			if panicked {
				results[idx].Result = err.Error()
			} else {
				results[idx].Result = fmt.Sprintf("tool call failed: %v", err)
			}
			span.RecordError(err)
		} else {
			results[idx].Result = truncateResult(out, policy.MaxResultChars)
		}

		span.SetAttributes(
			attrAgentToolDurationMs.Int64(results[idx].Duration.Milliseconds()),
			attrAgentToolError.Bool(err != nil),
		)

		if r.verbose {
			slog.DebugContext(ctx, "handling tool call",
				slog.String("name", call.Name),
				slog.String("call_id", call.Id),
				slog.String("call_args", truncateForLog(call.Arguments)),
				slog.String("call_result", truncateForLog(results[idx].Result)),
			)
		}

		toolCtxes[idx].Result = results[idx]
		if hookErrLocal := hooks.OnToolCallEnd(ctx, toolCtxes[idx]); hookErrLocal != nil {
			hookMu.Lock()
			if hookErr == nil {
				hookErr = pkgerrors.WithMessagef(hookErrLocal, "tool call end hook failed at %s", call.Name)
			}
			hookMu.Unlock()
		}
	}

	if policy.Serial {
		for idx := range calls {
			runOne(idx)
		}
	} else {
		var (
			sem chan struct{}
			wg  sync.WaitGroup
		)
		if policy.MaxConcurrency > 0 {
			sem = make(chan struct{}, policy.MaxConcurrency)
		}

		for idx := range calls {
			wg.Go(func() {
				if sem != nil {
					select {
					case sem <- struct{}{}:
						defer func() { <-sem }()
					case <-ctx.Done():
						results[idx].Error = ctx.Err()
						results[idx].Result = fmt.Sprintf("tool call failed: %v", ctx.Err())
						return
					}
				}

				runOne(idx)
			})
		}

		wg.Wait()
	}

	if hookErr != nil {
		return nil, hookErr
	}
	return results, nil
}

func findTool(entries []*toolEntry, name string) *toolEntry {
	for _, entry := range entries {
		if entry.info.Name == name {
			return entry
		}
	}
	return nil
}

func runTool(
	ctx context.Context,
	tool einotool.InvokableTool,
	name string,
	args string,
) (out string, err error, panicked bool) {
	defer func() {
		if p := recover(); p != nil {
			slog.ErrorContext(ctx, "tool call panic",
				slog.String("name", name),
				slog.Any("err", p),
				slog.String("stack", string(debug.Stack())),
			)
			err = fmt.Errorf("tool call panic: %v", p)
			panicked = true
		}
	}()

	out, err = tool.InvokableRun(ctx, args)
	return out, err, false
}

func truncateResult(s string, maxChars int) string {
	if maxChars <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	return string(runes[:maxChars]) + "\n...[tool result truncated]"
}

func truncateForLog(s string) string {
	runes := []rune(s)
	if len(runes) <= maxVerboseRune {
		return s
	}
	return string(runes[:maxVerboseRune]) + "(... truncated)"
}
