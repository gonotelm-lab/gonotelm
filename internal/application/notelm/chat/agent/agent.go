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
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/agentize"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgagt "github.com/gonotelm-lab/gonotelm/pkg/agent"
	"github.com/gonotelm-lab/gonotelm/pkg/llm"

	"github.com/bytedance/sonic"
	einotool "github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/pkg/errors"
)

type (
	ChatMessageStyle        string
	ChatMessageAnswerLength string
)

const (
	ChatMessageStyleDefault ChatMessageStyle = "default"
	ChatMessageStyleAnalyst ChatMessageStyle = "analyst"
	ChatMessageStyleGuide   ChatMessageStyle = "guide"

	ChatMessageAnswerLengthDefault ChatMessageAnswerLength = "default"
	ChatMessageAnswerLengthShorter ChatMessageAnswerLength = "shorter"
	ChatMessageAnswerLengthLonger  ChatMessageAnswerLength = "longer"
)

func (s ChatMessageStyle) IsValid() bool {
	switch s {
	case ChatMessageStyleDefault, ChatMessageStyleAnalyst, ChatMessageStyleGuide:
		return true
	default:
		return false
	}
}

func (l ChatMessageAnswerLength) IsValid() bool {
	switch l {
	case ChatMessageAnswerLengthDefault, ChatMessageAnswerLengthShorter, ChatMessageAnswerLengthLonger:
		return true
	default:
		return false
	}
}

// 聊天Agent
//
// 实现类似Agentic RAG回答问题
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
	Style           ChatMessageStyle
	AnswerLength    ChatMessageAnswerLength

	Model         string // model name
	ModelProvider string // model provider

	Hooks Hooks
}

type (
	RunResponse struct {
		SourceDocCitations []valobj.Id
		FinalMessage       *pkgagt.EinoMessage
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

	RoundFinishedHook func(ctx context.Context, newMsgs []*pkgagt.EinoMessage)

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

type ChatAgent = pkgagt.Agent[*SessionState]

func (a *Agent) prepareRun(req *RunRequest) (*ChatAgent, *SessionState, error) {
	toolCallingChatModel, err := a.gateway.GetProvider(chat.Provider(req.ModelProvider))
	if err != nil {
		return nil, nil, err
	}

	options := chat.BuildLLMOptions(
		chat.WithThinking(chat.Provider(req.ModelProvider), req.EnableThinking),
		chat.WithModel(req.Model),
	)
	session := &SessionState{
		chat:        req.Chat,
		notebook:    req.Notebook,
		sources:     req.Sources,
		userId:      req.UserId,
		curRunPhase: runPhase1,
	}
	domainAgent := pkgagt.New(pkgagt.Config[*SessionState]{
		MaxRound: conf.NotelmGlobal().Chat.GetMaxRound(),
		BaseLLM:  toolCallingChatModel,
		Options:  options,
		Verbose:  true,
	}, session)

	sourceIds := make([]valobj.Id, 0, len(req.Sources))
	for _, source := range req.Sources {
		sourceIds = append(sourceIds, source.Id)
	}

	// 绑定工具
	sourcePermissionChecker := a.isSourceAllowAccess(sourceIds)
	sourceDocPermissionChecker := a.isSourceDocAllowAccess(req.Notebook.Id, sourceIds)
	err = domainAgent.BindTools(map[string]einotool.InvokableTool{
		tools.GrepSourceToolName:  tools.NewGrepSourceTool(a.service, sourcePermissionChecker),
		tools.ReadSourceToolName:  tools.NewReadSourceTool(a.service, sourcePermissionChecker),
		tools.StatSourceToolName:  tools.NewStatSourceTool(a.service, sourcePermissionChecker),
		tools.QuerySourceToolName: tools.NewQuerySourceTool(a.service, req.Notebook.Id, sourcePermissionChecker),
		tools.MarkPhaseToolName:   tools.NewMarkPhaseTool(),
		tools.CiteSourceDocToolName: tools.NewCiteSourceDocTool(
			sourceDocPermissionChecker,
			tools.CitationCollectorFunc(func(sourceDocIds []valobj.Id) {
				session.sourceDocCitations = sourceDocIds
			}),
		),
	})
	if err != nil {
		return nil, nil, errors.WithMessage(err, "bind tools failed")
	}

	return domainAgent, session, nil
}

func (a *Agent) preparePrompt(ctx context.Context, domainAgent *ChatAgent, req *RunRequest) error {
	promptVars, err := a.buildPromptVars(req)
	if err != nil {
		return errors.WithMessage(err, "build prompt vars failed")
	}
	systemPrompt, err := renderSystemPrompt(ctx, promptVars)
	if err != nil {
		return errors.WithMessage(err, "render system prompt failed")
	}

	// set callbacks
	domainAgent.OnBeforeChat(func(
		ctx context.Context,
		state *SessionState,
		msgs []*einoschema.Message,
	) ([]*einoschema.Message, error) {
		newMsgs := make([]*einoschema.Message, 0, len(msgs)+1)
		newMsgs = append(newMsgs, systemPrompt)
		newMsgs = append(newMsgs, msgs...)
		return newMsgs, nil
	})

	return nil
}

// Run 是底层大模型为纯流式输出 大模型的每个输出都直接反映到设置的回调函数上
//
// Deprecated
func (a *Agent) Run(ctx context.Context, req *RunRequest) (*RunResponse, error) {
	domainAgent, session, err := a.prepareRun(req)
	if err != nil {
		return nil, errors.WithMessagef(err, "prepare run agent failed")
	}

	err = a.preparePrompt(ctx, domainAgent, req)
	if err != nil {
		return nil, errors.WithMessagef(err, "prepare prompt failed")
	}
	a.bindHooks(domainAgent, req)

	ctxMsgs := make([]*einoschema.Message, 0, len(req.ContextMessages))
	for _, msg := range req.ContextMessages {
		ctxMsgs = append(ctxMsgs, msg.Message)
	}
	final, err := domainAgent.ReactStream(ctx, ctxMsgs)
	if err != nil {
		return nil, errors.WithMessage(err, "agent react stream failed")
	}

	return &RunResponse{
		SourceDocCitations: session.sourceDocCitations,
		FinalMessage:       final,
	}, nil
}

// RunV2 底层不再完全使用大模型流式输出 只在最后的关键节点输出才使用流式输出 其它都是非流式
// 即RunV2先非流式 再流式
func (a *Agent) RunV2(ctx context.Context, req *RunRequest) (*RunResponse, error) {
	domainAgent, session, err := a.prepareRun(req)
	if err != nil {
		return nil, errors.WithMessagef(err, "prepare run agent failed")
	}

	err = a.preparePrompt(ctx, domainAgent, req)
	if err != nil {
		return nil, errors.WithMessagef(err, "prepare prompt failed")
	}
	a.bindHooksV2(domainAgent, req)

	ctxMsgs := make([]*einoschema.Message, 0, len(req.ContextMessages))
	for _, msg := range req.ContextMessages {
		ctxMsgs = append(ctxMsgs, msg.Message)
	}
	session.enterRunPhase1()
	phase1FinalMsg, err := domainAgent.React(ctx, ctxMsgs) // 此时只是中间结果
	if err != nil {
		return nil, errors.WithMessage(err, "intermediate agent react failed")
	}

	if phase1FinalMsg != nil &&
		phase1FinalMsg.ResponseMeta != nil &&
		phase1FinalMsg.ResponseMeta.FinishReason == llm.FinishReasonStop &&
		!session.finalPhaseMarked {
		// 此处兼容一种情况 如果phase1直接输出了finish_stop 意味着模型没有严格遵守要求直接输出了答案
		// 那就直接推送 不要浪费token了 手动触发一次hook
		slog.DebugContext(ctx, "runV2 phase1 directly finish_reason=stop, manual pushing...")
		a.manualPushFinalMsg(ctx, phase1FinalMsg, &req.Hooks)
		return &RunResponse{
			FinalMessage:       phase1FinalMsg,
			SourceDocCitations: session.sourceDocCitations,
		}, nil
	}

	slog.DebugContext(ctx, "runV2 after phase1 react, now reactStream")

	// 第二个阶段开始流式输出
	session.enterRunPhase2()
	finalMsg, err := domainAgent.ReactStream(ctx, []*pkgagt.EinoMessage{{
		Role:    einoschema.User,
		Content: "Please output the final answer now.",
	}})
	if err != nil {
		return nil, errors.WithMessage(err, "final agent react stream failed")
	}

	return &RunResponse{
		FinalMessage:       finalMsg,
		SourceDocCitations: session.sourceDocCitations,
	}, nil
}

func (a *Agent) manualPushFinalMsg(ctx context.Context, msg *pkgagt.EinoMessage, hooks *Hooks) {
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

func (a *Agent) bindBasicHooks(domainAgent *ChatAgent, req *RunRequest) {
	domainAgent.OnBeforeRound(pkgagt.NewFinalRoundHook(domainAgent, conf.NotelmGlobal().Chat.GetMaxRound()))

	if req.Hooks.RoundFinishedHook != nil {
		domainAgent.OnMsgAppender(func(ctx context.Context, state *SessionState, newMsgs []*pkgagt.EinoMessage) {
			req.Hooks.RoundFinishedHook(ctx, newMsgs)
		})
	}

	if req.Hooks.ThinkStart != nil {
		domainAgent.OnReasoningStart(func(ctx context.Context, round int, state *SessionState) error {
			req.Hooks.ThinkStart(ctx)
			return nil
		})
	}
	if req.Hooks.ThinkDelta != nil {
		domainAgent.OnReasoningDelta(func(ctx context.Context, round int, state *SessionState, delta string) error {
			req.Hooks.ThinkDelta(ctx, delta)
			return nil
		})
	}
	if req.Hooks.ThinkEnd != nil {
		domainAgent.OnReasoningEnd(func(ctx context.Context, round int, state *SessionState) error {
			req.Hooks.ThinkEnd(ctx)
			return nil
		})
	}

	if req.Hooks.ResponseStart != nil {
		domainAgent.OnContentStart(func(ctx context.Context, round int, state *SessionState) error {
			req.Hooks.ResponseStart(ctx)
			return nil
		})
	}
	if req.Hooks.ResponseDelta != nil {
		domainAgent.OnContentDelta(func(ctx context.Context, round int, state *SessionState, delta string) error {
			req.Hooks.ResponseDelta(ctx, delta)
			return nil
		})
	}
	if req.Hooks.ResponseEnd != nil {
		domainAgent.OnContentEnd(func(ctx context.Context, round int, state *SessionState) error {
			req.Hooks.ResponseEnd(ctx)
			return nil
		})
	}

	domainAgent.OnAfterToolCall(func(ctx context.Context, state *SessionState, results []*pkgagt.ToolCallResult) {
		// 额外处理工具调用结果
		for _, result := range results {
			switch result.Name {
			case tools.MarkPhaseToolName:
				var input tools.MarkPhaseToolInput
				if err := sonic.Unmarshal([]byte(result.Arguments), &input); err == nil {
					if req.Hooks.PhaseMarkHook != nil {
						state.finalPhaseMarked = input.IsFinal
						req.Hooks.PhaseMarkHook(ctx, Phase{
							Summary:     input.Summary,
							Description: input.Description,
							IsFinal:     input.IsFinal,
						})
					}
				}
			}
		}
	})
}

func (a *Agent) bindHooks(domainAgent *ChatAgent, req *RunRequest) {
	a.bindBasicHooks(domainAgent, req)
}

func (a *Agent) bindHooksV2(domainAgent *ChatAgent, req *RunRequest) {
	a.bindBasicHooks(domainAgent, req)

	domainAgent.OnAfterRound(func(ctx context.Context, round int, state *SessionState, roundMsg *pkgagt.EinoMessage) (bool, error) {
		// 工具调用中出现了最后一个就Phase提前结束
		if state.isInRunPhase1() && state.finalPhaseMarked {
			return true, nil
		}
		return false, nil
	})
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
func (a *Agent) isSourceAllowAccess(allowedSourceIds []valobj.Id) tools.SourcePermissionChecker {
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
func (a *Agent) isSourceDocAllowAccess(
	notebookId valobj.Id,
	sourceIds []valobj.Id,
) tools.SourceDocPermissionChecker {
	return tools.SourceDocPermissionCheckerFunc(func(ctx context.Context, sourceDocIds []valobj.Id) error {
		return a.service.CheckSourceDocAllowAccess(ctx, notebookId, sourceIds, sourceDocIds)
	})
}
