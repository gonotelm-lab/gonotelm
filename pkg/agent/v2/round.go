package agent

import (
	"context"
	"log/slog"
	"strings"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/llm"
	"github.com/gonotelm-lab/gonotelm/pkg/trace"
)

type roundOutcome struct {
	assistant     *schema.Message
	toolResults   []ToolResult
	finishReason  string
	usage         schema.TokenUsage
	stop          bool
	contentFilter bool
}

// Run 使用非流式模型调用执行一次 agent 运行。
func (r *Runner[S]) Run(
	ctx context.Context,
	s *Session[S],
	cfg RunConfig[S],
) (*Result, error) {
	return r.runLoop(ctx, s, cfg, generateTransport[S]{})
}

// RunStream 使用流式模型调用执行一次 agent 运行。
func (r *Runner[S]) RunStream(
	ctx context.Context,
	s *Session[S],
	cfg RunConfig[S],
) (*Result, error) {
	return r.runLoop(ctx, s, cfg, streamTransport[S]{})
}

func (r *Runner[S]) runLoop(
	ctx context.Context,
	s *Session[S],
	cfg RunConfig[S],
	tr transport[S],
) (res *Result, err error) {
	if s == nil {
		return nil, pkgerrors.ErrParams.Msg("agent session is required")
	}

	hooks := hooksOrDefault(cfg.Hooks)

	maxRound := cfg.MaxRound
	if maxRound <= 0 {
		maxRound = r.maxRound
	}

	opts := make([]einomodel.Option, 0, len(r.opts)+len(cfg.Options))
	opts = append(opts, r.opts...)
	opts = append(opts, cfg.Options...)

	ctx, span := trace.GetOtelTracer().Start(
		ctx,
		spanAgentRun,
		oteltrace.WithAttributes(
			attrAgentName.String(r.name),
			attrAgentMaxRound.Int(maxRound),
		),
	)

	startUsage := s.Usage
	lastRound := 0
	defer func() {
		hooks.OnRunEnd(ctx, &RunContext[S]{
			Session: s,
			Info:    RunInfo{Agent: r.name, Round: lastRound, MaxRound: maxRound},
			Result:  res,
			Err:     err,
		})
		r.logRunSummary(ctx, res)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	if err := hooks.OnRunStart(ctx, &RunContext[S]{
		Session: s,
		Info:    RunInfo{Agent: r.name, MaxRound: maxRound},
	}); err != nil {
		return nil, pkgerrors.WithMessage(err, "run start hook failed")
	}

	totalToolCalls := 0
	for round := 0; round < maxRound; round++ {
		lastRound = round
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		outcome, roundErr := r.runRound(ctx, s, cfg, hooks, tr, round, maxRound, opts)
		if outcome != nil {
			if berr := cfg.Budget.AddUsage(outcome.usage); berr != nil {
				return r.buildResult(s, outcome, StopFinal, round+1, totalToolCalls, startUsage), berr
			}
		}
		if roundErr != nil {
			if outcome != nil {
				return r.buildResult(s, outcome, StopFinal, round+1, totalToolCalls, startUsage), roundErr
			}
			return nil, roundErr
		}

		totalToolCalls += len(outcome.toolResults)

		switch {
		case outcome.contentFilter:
			return r.buildResult(s, outcome, StopContentFilter, round+1, totalToolCalls, startUsage), nil
		case outcome.stop:
			return r.buildResult(s, outcome, StopEarly, round+1, totalToolCalls, startUsage), nil
		case len(outcome.assistant.ToolCalls) == 0:
			return r.buildResult(s, outcome, StopFinal, round+1, totalToolCalls, startUsage), nil
		}
	}

	return nil, ErrMaxRounds.Msgf("agent exceeded max rounds=%d", maxRound)
}

func (r *Runner[S]) runRound(
	ctx context.Context,
	s *Session[S],
	cfg RunConfig[S],
	hooks Hooks[S],
	tr transport[S],
	round int,
	maxRound int,
	opts []einomodel.Option,
) (*roundOutcome, error) {
	info := RunInfo{Agent: r.name, Round: round, MaxRound: maxRound}

	ctx, span := trace.GetOtelTracer().Start(
		ctx,
		spanAgentRound,
		oteltrace.WithAttributes(
			attrAgentName.String(r.name),
			attrAgentRound.Int(round),
			attrAgentMaxRound.Int(maxRound),
		),
	)
	defer span.End()

	rc := &RoundContext[S]{Session: s, Info: info}
	if err := hooks.OnRoundStart(ctx, rc); err != nil {
		return nil, pkgerrors.WithMessagef(err, "round %d start hook failed", round)
	}

	tools, err := r.resolvePlanTools(ctx, cfg, rc.Plan)
	if err != nil {
		return nil, err
	}

	baseModel := r.model
	if cfg.Model != nil {
		baseModel = cfg.Model
	}
	if rc.Plan.Model != nil {
		baseModel = rc.Plan.Model
	}
	model, err := r.resolveModel(baseModel, tools)
	if err != nil {
		return nil, err
	}

	msgs := composeMessages(s, rc.Plan.Inject)

	assistant, err := tr.call(ctx, model, msgs, opts, hooks, info)
	if err != nil {
		return nil, err
	}

	usage := assistantUsage(assistant)
	addUsage(&s.Usage, usage)

	finishReason := ""
	if assistant.ResponseMeta != nil {
		finishReason = assistant.ResponseMeta.FinishReason
	}
	span.SetAttributes(
		attrGenAIFinishReason.String(finishReason),
		attrGenAIPromptTokens.Int(usage.PromptTokens),
		attrGenAICompletionTokens.Int(usage.CompletionTokens),
	)

	outcome := &roundOutcome{
		assistant:    assistant,
		finishReason: finishReason,
		usage:        usage,
	}

	if finishReason == llm.FinishReasonContentFilter {
		s.Append(assistant)
		rc.Result = RoundResult{
			Assistant:   assistant,
			NewMessages: []*schema.Message{assistant},
		}
		rc.Msg = assistant
		if err := hooks.OnRoundComplete(ctx, rc); err != nil {
			return outcome, pkgerrors.WithMessagef(err, "round %d complete hook failed", round)
		}
		if err := hooks.OnRoundEnd(ctx, rc); err != nil {
			return outcome, pkgerrors.WithMessagef(err, "round %d end hook failed", round)
		}
		outcome.contentFilter = true
		return outcome, nil
	}

	var toolMsgs []*schema.Message
	if len(assistant.ToolCalls) > 0 {
		if err := cfg.Budget.AddToolCallFor(len(assistant.ToolCalls)); err != nil {
			return outcome, err
		}

		policy := r.policy
		if rc.Plan.ToolPolicy != nil {
			policy = *rc.Plan.ToolPolicy
		}

		outcome.toolResults, err = r.executeTools(
			ctx, s, toolCallsOf(assistant), tools, hooks, info, policy,
		)
		if err != nil {
			return outcome, err
		}
		toolMsgs = toolMessages(outcome.toolResults)
	}

	appended := make([]*schema.Message, 0, 1+len(toolMsgs))
	appended = append(appended, assistant)
	appended = append(appended, toolMsgs...)
	s.Append(appended...)

	rc.Result = RoundResult{
		Assistant:   assistant,
		ToolResults: outcome.toolResults,
		NewMessages: appended,
	}
	if err := hooks.OnRoundComplete(ctx, rc); err != nil {
		return outcome, pkgerrors.WithMessagef(err, "round %d complete hook failed", round)
	}

	rc.Msg = assistant
	if err := hooks.OnRoundEnd(ctx, rc); err != nil {
		return outcome, pkgerrors.WithMessagef(err, "round %d end hook failed", round)
	}
	outcome.stop = rc.Stop

	return outcome, nil
}

func (r *Runner[S]) resolvePlanTools(
	ctx context.Context,
	cfg RunConfig[S],
	plan RoundPlan,
) ([]*toolEntry, error) {
	if plan.NoTools || cfg.WithoutTools {
		return nil, nil
	}
	if plan.Tools != nil {
		return validateTools(ctx, plan.Tools)
	}
	return r.resolveTools(ctx, cfg)
}

func (r *Runner[S]) buildResult(
	s *Session[S],
	outcome *roundOutcome,
	stop StopReason,
	rounds int,
	toolCalls int,
	startUsage schema.TokenUsage,
) *Result {
	return &Result{
		Final:        outcome.assistant,
		Stop:         stop,
		FinishReason: outcome.finishReason,
		Rounds:       rounds,
		ToolCalls:    toolCalls,
		Usage:        diffUsage(s.Usage, startUsage),
	}
}

func (r *Runner[S]) logRunSummary(ctx context.Context, res *Result) {
	if res == nil {
		return
	}
	slog.InfoContext(ctx, "agent run finished",
		slog.String("agent", r.name),
		slog.Int("rounds", res.Rounds),
		slog.Int("tool_calls", res.ToolCalls),
		slog.Any("usage", res.Usage),
		slog.Int("stop", int(res.Stop)),
	)
}

func composeMessages[S any](
	s *Session[S],
	inject []*schema.Message,
) []*schema.Message {
	capacity := len(s.Messages) + len(inject) + 1
	msgs := make([]*schema.Message, 0, capacity)

	if strings.TrimSpace(s.System) != "" {
		msgs = append(msgs, schema.SystemMessage(s.System))
	}
	msgs = append(msgs, s.Messages...)
	msgs = append(msgs, inject...)

	return msgs
}

func assistantUsage(msg *schema.Message) schema.TokenUsage {
	if msg == nil || msg.ResponseMeta == nil || msg.ResponseMeta.Usage == nil {
		return schema.TokenUsage{}
	}
	return *msg.ResponseMeta.Usage
}

func addUsage(dst *schema.TokenUsage, usage schema.TokenUsage) {
	dst.PromptTokens += usage.PromptTokens
	dst.PromptTokenDetails.CachedTokens += usage.PromptTokenDetails.CachedTokens
	dst.CompletionTokens += usage.CompletionTokens
	dst.CompletionTokensDetails.ReasoningTokens += usage.CompletionTokensDetails.ReasoningTokens
	dst.TotalTokens += usage.TotalTokens
}

func diffUsage(after, before schema.TokenUsage) schema.TokenUsage {
	return schema.TokenUsage{
		PromptTokens:     after.PromptTokens - before.PromptTokens,
		CompletionTokens: after.CompletionTokens - before.CompletionTokens,
		TotalTokens:      after.TotalTokens - before.TotalTokens,
		PromptTokenDetails: schema.PromptTokenDetails{
			CachedTokens: after.PromptTokenDetails.CachedTokens - before.PromptTokenDetails.CachedTokens,
		},
		CompletionTokensDetails: schema.CompletionTokensDetails{
			ReasoningTokens: after.CompletionTokensDetails.ReasoningTokens - before.CompletionTokensDetails.ReasoningTokens,
		},
	}
}

func tokenTotal(usage schema.TokenUsage) int {
	if usage.TotalTokens > 0 {
		return usage.TotalTokens
	}
	return usage.PromptTokens + usage.CompletionTokens
}

func toolCallsOf(msg *schema.Message) []ToolCall {
	calls := make([]ToolCall, 0, len(msg.ToolCalls))
	for _, call := range msg.ToolCalls {
		calls = append(calls, ToolCall{
			Id:        call.ID,
			Name:      call.Function.Name,
			Arguments: call.Function.Arguments,
		})
	}
	return calls
}

func toolMessages(results []ToolResult) []*schema.Message {
	msgs := make([]*schema.Message, 0, len(results))
	for _, result := range results {
		msgs = append(msgs, schema.ToolMessage(result.Result, result.Id, schema.WithToolName(result.Name)))
	}
	return msgs
}
