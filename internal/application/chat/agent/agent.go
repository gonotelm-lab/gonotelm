package agent

import (
	"context"
	"slices"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/chat/agent/tools"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	domainagent "github.com/gonotelm-lab/gonotelm/pkg/agent"
	chatentity "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/agentize"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"

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
	UserId          string
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
		FinalMessage       *domainagent.EinoMessage
	}

	Phase struct {
		Summary     string
		Description string
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

	RoundFinishedHook func(ctx context.Context, newMsgs []*domainagent.EinoMessage)

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

func (a *Agent) Run(ctx context.Context, req *RunRequest) (*RunResponse, error) {
	toolCallingChatModel, err := a.gateway.GetProvider(llm.Provider(req.ModelProvider))
	if err != nil {
		return nil, err
	}

	options := chat.BuildLLMOptions(
		chat.WithThinking(llm.Provider(req.ModelProvider), req.EnableThinking),
		chat.WithModel(req.Model),
	)
	session := &SessionState{
		chat:     req.Chat,
		notebook: req.Notebook,
		sources:  req.Sources,
		userId:   req.UserId,
	}
	domainAgent := domainagent.New(domainagent.Config[*SessionState]{
		MaxRound: conf.Global().Logic.Chat.GetMaxRound(),
		BaseLLM:  toolCallingChatModel,
		Options:  options,
		// Verbose:  true,
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
		return nil, errors.WithMessage(err, "bind tools failed")
	}

	promptVars, err := a.buildPromptVars(req)
	if err != nil {
		return nil, errors.WithMessage(err, "build prompt vars failed")
	}
	systemPrompt, err := renderSystemPrompt(ctx, promptVars)
	if err != nil {
		return nil, errors.WithMessage(err, "render system prompt failed")
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

func (a *Agent) bindHooks(domainAgent *domainagent.Agent[*SessionState], req *RunRequest) {
	// TODO
	domainAgent.OnBeforeRound(nil) // 如果只剩下最后一轮 去掉所有工具 要求模型立马输出

	if req.Hooks.RoundFinishedHook != nil {
		domainAgent.OnMsgAppender(func(ctx context.Context, state *SessionState, newMsgs []*domainagent.EinoMessage) {
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

	domainAgent.OnAfterToolCall(func(ctx context.Context, state *SessionState, results []*domainagent.ToolCallResult) {
		// 额外处理工具调用结果
		for _, result := range results {
			switch result.Name {
			case tools.MarkPhaseToolName:
				var input tools.MarkPhaseToolInput
				if err := sonic.Unmarshal([]byte(result.Arguments), &input); err == nil {
					req.Hooks.PhaseMarkHook(ctx, Phase{
						Summary:     input.Summary,
						Description: input.Description,
					})
				}
			}
		}
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
