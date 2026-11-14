package agent

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/agent/tools"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatentity "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chatvo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/vo"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/agentize"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	agentv2 "github.com/gonotelm-lab/gonotelm/pkg/agent/v2"
	"github.com/gonotelm-lab/gonotelm/pkg/llm"

	einoschema "github.com/cloudwego/eino/schema"
	"github.com/pkg/errors"
)

const (
	agentName = "chat"

	// finalAnswerPrompt 是阶段二的开场指令：探索已经结束，只输出最终回答。
	finalAnswerPrompt = "Please output the final answer now."
)

// 聊天Agent
//
// 实现类似Agentic RAG回答问题
//
// 执行层由 Runner 与 Session 组成：Runner 持有模型与工具，Session 持有业务状态、系统提示词与上下文消息。
type Agent struct {
	service      *agentize.Service
	gateway      *chat.Gateway
	sourceRepo   sourcerepo.Repository
	notebookRepo notebookrepo.Repository
}

func New(
	s *agentize.Service,
	g *chat.Gateway,
	sourceRepo sourcerepo.Repository,
	notebookRepo notebookrepo.Repository,
) *Agent {
	return &Agent{
		service:      s,
		gateway:      g,
		sourceRepo:   sourceRepo,
		notebookRepo: notebookRepo,
	}
}

type RunRequest struct {
	Notebook        *notebookentity.Notebook
	Chat            *chatentity.Chat
	UserId          valobj.Uid
	ContextMessages []*chatentity.ContextMessage
	Sources         []*sourceentity.Source
	EnableThinking  bool
	Style           chatvo.Style
	AnswerLength    chatvo.AnswerLength

	Model         string // model name
	ModelProvider string // model provider

	Hooks Hooks
}

type (
	RunResponse struct {
		SourceDocCitations []valobj.Id
		FinalMessage       *einoschema.Message
	}

	Phase struct {
		Summary     string
		Description string
		IsFinal     bool
	}

	Citation struct {
		SourceDocIds []valobj.Id
	}

	ThinkStartHook func(ctx context.Context)
	ThinkDeltaHook func(ctx context.Context, content string)
	ThinkEndHook   func(ctx context.Context)

	ResponseStartHook func(ctx context.Context)
	ResponseDeltaHook func(ctx context.Context, delta string)
	ResponseEndHook   func(ctx context.Context)

	PhaseMarkHook func(ctx context.Context, phase Phase)

	RoundFinishedHook func(ctx context.Context, newMsgs []*einoschema.Message)

	Hooks struct {
		ThinkStart        ThinkStartHook
		ThinkDelta        ThinkDeltaHook
		ThinkEnd          ThinkEndHook
		ResponseStart     ResponseStartHook
		ResponseDelta     ResponseDeltaHook
		ResponseEnd       ResponseEndHook
		PhaseMarkHook     PhaseMarkHook
		RoundFinishedHook RoundFinishedHook
	}
)

// prepareRun 构造本次运行所需的 Runner 与 Session。
//
// Runner 只装模型、工具和轮数上限，可以跨会话复用；Session 承载本次运行的状态、
// 系统提示词和上下文消息，只允许单 goroutine 持有。
func (a *Agent) prepareRun(
	ctx context.Context,
	req *RunRequest,
) (*agentv2.Runner[*SessionState], *agentv2.Session[*SessionState], error) {
	tcm, err := a.gateway.GetChatModel(chat.Provider(req.ModelProvider))
	if err != nil {
		return nil, nil, err
	}

	options := chat.BuildLLMOptions(
		chat.WithThinking(chat.Provider(req.ModelProvider), req.EnableThinking),
		chat.WithModel(req.Model),
	)

	state := &SessionState{
		chat:        req.Chat,
		notebook:    req.Notebook,
		sources:     req.Sources,
		userId:      req.UserId,
		curRunPhase: runPhase1,
	}
	session := agentv2.NewSession(state)

	systemPrompt, err := a.buildSystemPrompt(ctx, req)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "build system prompt failed")
	}
	session.System = systemPrompt

	ctxMsgs := make([]*einoschema.Message, 0, len(req.ContextMessages))
	for _, msg := range req.ContextMessages {
		if msg == nil || msg.Message == nil {
			continue
		}
		ctxMsgs = append(ctxMsgs, msg.Message)
	}
	session.Append(ctxMsgs...)

	runner, err := agentv2.NewRunner[*SessionState](ctx, agentv2.RunnerConfig{
		Name:     agentName,
		Model:    tcm,
		Options:  options,
		Tools:    a.buildTools(state, req),
		MaxRound: conf.NotelmGlobal().Chat.GetMaxRound(),
	})
	if err != nil {
		return nil, nil, errors.WithMessage(err, "bind tools failed")
	}

	return runner, session, nil
}

// buildTools 装配本次运行的工具集，权限检查与引用收集都绑定到本次运行的 state 上。
func (a *Agent) buildTools(state *SessionState, req *RunRequest) []agentv2.Tool {
	sourceIds := make([]valobj.Id, 0, len(req.Sources))
	for _, source := range req.Sources {
		sourceIds = append(sourceIds, source.Id)
	}

	sourcePermissionChecker := a.composeSourceAllowAccessTool(sourceIds)
	sourceDocPermissionChecker := a.composeSourceDocAllowAccessTool(req.Notebook.Id, sourceIds)

	return []agentv2.Tool{
		tools.NewGrepSourceTool(a.service, sourcePermissionChecker),
		tools.NewReadSourceTool(a.service, sourcePermissionChecker),
		tools.NewStatSourceTool(a.service, sourcePermissionChecker),
		tools.NewQuerySourceTool(a.service, req.Notebook.Id, sourcePermissionChecker),
		tools.NewMarkPhaseTool(),
		tools.NewCiteSourceDocTool(sourceDocPermissionChecker, composeCitationCollector(state)),
	}
}

func (a *Agent) buildSystemPrompt(ctx context.Context, req *RunRequest) (string, error) {
	promptVars, err := a.buildPromptVars(req)
	if err != nil {
		return "", errors.WithMessage(err, "build prompt vars failed")
	}

	systemPrompt, err := renderSystemPrompt(ctx, promptVars)
	if err != nil {
		return "", errors.WithMessage(err, "render system prompt failed")
	}

	return systemPrompt.Content, nil
}

// RunV2 分两段执行：阶段一非流式探索（工具、阶段标记、引用登记），
// 阶段二流式输出最终回答，只有阶段二的增量会推送给客户端。
func (a *Agent) RunV2(ctx context.Context, req *RunRequest) (*RunResponse, error) {
	runner, session, err := a.prepareRun(ctx, req)
	if err != nil {
		return nil, errors.WithMessage(err, "prepare run agent failed")
	}

	return runChat(ctx, runner, session, &req.Hooks)
}

// runChat 是 RunV2 的实际执行流程，独立于 Agent 便于直接驱动 Runner 做测试。
func runChat(
	ctx context.Context,
	runner *agentv2.Runner[*SessionState],
	session *agentv2.Session[*SessionState],
	hooks *Hooks,
) (*RunResponse, error) {
	state := session.State
	state.enterRunPhase1()

	res, err := runner.Run(ctx, session, agentv2.RunConfig[*SessionState]{
		Hooks: newChatHooks(hooks, false, true), // 阶段一不推送增量
	})
	if err != nil {
		return nil, errors.WithMessage(err, "intermediate agent react failed")
	}

	phase1FinalMsg := res.Final
	if phase1FinalMsg != nil &&
		phase1FinalMsg.ResponseMeta != nil &&
		phase1FinalMsg.ResponseMeta.FinishReason == llm.FinishReasonStop &&
		!state.finalPhaseMarked {
		// 此处兼容一种情况 如果phase1直接输出了finish_stop 意味着模型没有严格遵守要求直接输出了答案
		// 那就直接推送 不要浪费token了 手动触发一次hook
		slog.DebugContext(ctx, "runV2 phase1 directly finish_reason=stop, manual pushing...")
		manualPushFinalMsg(ctx, phase1FinalMsg, hooks)
		return &RunResponse{
			FinalMessage:       phase1FinalMsg,
			SourceDocCitations: state.sourceDocCitations,
		}, nil
	}

	slog.DebugContext(ctx, "runV2 after phase1 react, now reactStream")

	// 第二个阶段开始流式输出
	state.enterRunPhase2()
	session.Append(einoschema.UserMessage(finalAnswerPrompt))

	res, err = runner.RunStream(ctx, session, agentv2.RunConfig[*SessionState]{
		Hooks: newChatHooks(hooks, true, false), // 阶段二推送增量
	})
	if err != nil {
		return nil, errors.WithMessage(err, "final agent react stream failed")
	}

	return &RunResponse{
		FinalMessage:       res.Final,
		SourceDocCitations: state.sourceDocCitations,
	}, nil
}

// manualPushFinalMsg 把阶段一直接产出的最终回答按「开始-增量-结束」喂给业务 hooks。
func manualPushFinalMsg(ctx context.Context, msg *einoschema.Message, hooks *Hooks) {
	if msg.Content == "" {
		return
	}

	if hooks.ResponseStart != nil {
		hooks.ResponseStart(ctx)
	}

	if hooks.ResponseDelta != nil {
		for chunk := range strings.SplitAfterSeq(msg.Content, "\n") {
			hooks.ResponseDelta(ctx, chunk)
		}
	}

	if hooks.ResponseEnd != nil {
		hooks.ResponseEnd(ctx)
	}
}

func (a *Agent) buildPromptVars(req *RunRequest) (PromptTemplateVars, error) {
	vars := PromptTemplateVars{
		Style:        req.Style,
		AnswerLength: req.AnswerLength,
	}

	vars.Notebook = formatNotebookInfo(req.Notebook.Name, req.Notebook.Description)
	if len(req.Sources) == 0 {
		return vars, nil
	}

	vars.Sources = make([]PromptSource, 0, len(req.Sources))
	for _, source := range req.Sources {
		vars.Sources = append(vars.Sources, PromptSource{
			Id:       source.Id.String(),
			Name:     strings.TrimSpace(source.Title),
			Abstract: strings.TrimSpace(source.Abstract),
		})
	}

	return vars, nil
}

// 检查sourceIds是否可以被当前Agent访问
func (a *Agent) composeSourceAllowAccessTool(allowedSourceIds []valobj.Id) tools.SourcePermissionChecker {
	return tools.SourcePermissionCheckerFunc(func(ctx context.Context, sourceIds []valobj.Id) error {
		for _, sourceId := range sourceIds {
			if !slices.Contains(allowedSourceIds, sourceId) {
				return errors.Errorf("source %s not allowed", sourceId.String())
			}
		}

		return nil
	})
}

// 检查sourceDocIds是否可以被当前Agent访问
func (a *Agent) composeSourceDocAllowAccessTool(
	notebookId valobj.Id,
	sourceIds []valobj.Id,
) tools.SourceDocPermissionChecker {
	return tools.SourceDocPermissionCheckerFunc(func(ctx context.Context, sourceDocIds []valobj.Id) error {
		return a.service.CheckSourceDocAllowAccess(ctx, notebookId, sourceIds, sourceDocIds)
	})
}

// composeCitationCollector 把 CiteSourceDoc 的登记结果写回本次运行的状态，供落库引用使用。
func composeCitationCollector(state *SessionState) tools.CitationCollector {
	return tools.CitationCollectorFunc(func(sourceDocIds []valobj.Id) {
		state.sourceDocCitations = sourceDocIds
		state.citationRegistered = true
	})
}
