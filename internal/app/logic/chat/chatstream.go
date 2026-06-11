package chat

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	bizagent "github.com/gonotelm-lab/gonotelm/internal/app/agent"
	bizchat "github.com/gonotelm-lab/gonotelm/internal/app/biz/chat"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	chatmodel "github.com/gonotelm-lab/gonotelm/internal/app/model/chat"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	einoschema "github.com/cloudwego/eino/schema"
)

func (l *Logic) processUserMessageTask(
	ctx context.Context,
	chat *chatmodel.Chat,
	taskId string,
	msgId uuid.UUID,
	params *CreateUserMessageParams,
) {
	userId := pkgcontext.GetUserId(ctx)
	sessionState := &chatSessionState{
		id:               0, // accumulated id
		taskId:           taskId,
		chatId:           chat.Id,
		notebookId:       chat.NotebookId,
		sourceIds:        params.SourceIds,
		userId:           userId,
		userLang:         pkgcontext.GetLang(ctx),
		enableThinking:   params.EnableThinking,
		chatStyle:        params.ChatStyle,
		chatAnswerLength: params.ChatAnswerLength,
	}
	ctx, sessionState.cancel = context.WithCancel(ctx)

	var (
		doFinalizing            bool = true
		answer                  *einoschema.Message
		err                     error
		errorContent            string = "服务繁忙，请稍后重试"
		addMessageWhenErr bool   = true
	)

	defer func() {
		if e := recover(); e != nil {
			stacks := debug.Stack()
			slog.ErrorContext(ctx, "background process user message panic",
				slog.Any("err", e),
				slog.String("stack", string(stacks)),
			)
			err = errors.WithStack(fmt.Errorf("panic: %v", e))
			addMessageWhenErr = false
		}

		if doFinalizing {
			if err := l.finalizingProcess(
				ctx,
				sessionState,
				answer,
				err,
				addMessageWhenErr,
				errorContent,
			); err != nil {
				slog.ErrorContext(ctx, "finalizing process failed",
					slog.String("chat_id", chat.Id.String()), slog.Any("err", err),
				)
			}
		}
	}()

	if !l.isUserMessageExist(ctx, chat.Id, msgId) {
		addMessageWhenErr = false
		return
	}
	var queriedSourceDocs []*model.SourceDoc
	if len(params.SourceIds) > 0 {
		l.emitRetrievingStreamEvent(ctx, sessionState, true)
		queriedSourceDocs, err = l.processRetrievingSourceDocs(
			ctx,
			chat.NotebookId,
			params,
			taskId,
		)
		if err != nil {
			addMessageWhenErr = false
			return
		}
	}

	sessionState.sourceDocs = queriedSourceDocs
	if len(queriedSourceDocs) > 0 {
		l.emitRetrievingStreamEvent(ctx, sessionState, true)
	}

	var contextMsgs []*einoschema.Message
	contextMsgs, err = l.checkMessageContexts(ctx, chat.Id)
	if err != nil {
		addMessageWhenErr = false
		return
	}

	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.ChatScene)
	chatAgent, err := l.buildChatAgent(ctx, chat.Id, sessionState)
	if err != nil {
		addMessageWhenErr = true
		return
	}

	answer, err = chatAgent.ReactStream(ctx, contextMsgs)
	if err != nil {
		slog.ErrorContext(ctx, "chat agent loop failed",
			slog.String("chat_id", chat.Id.String()),
			slog.Any("err", err),
		)
	}
}

// 检查目标消息是否存在
func (l *Logic) isUserMessageExist(ctx context.Context, chatId, msgId uuid.UUID) bool {
	logAttrs := func(err error) []any {
		attrs := []any{
			slog.String("msg_id", msgId.String()),
			slog.String("chat_id", chatId.String()),
		}
		if err != nil {
			attrs = append(attrs, slog.Any("err", err))
		}

		return attrs
	}

	// get msg first
	targetMsg, err := l.chatBiz.GetMessage(ctx, msgId, chatId)
	if err != nil {
		if errors.Is(err, bizchat.ErrChatMessageNotFound) {
			return false
		}

		slog.ErrorContext(ctx, "get message failed", logAttrs(err)...)
		return false
	}

	if targetMsg.MsgRole != chatmodel.MessageRoleUser {
		slog.WarnContext(ctx, "target message is not a user message", logAttrs(nil)...)
		return false
	}

	return true
}


func (l *Logic) checkMessageContexts(
	ctx context.Context,
	chatId uuid.UUID,
) ([]*einoschema.Message, error) {
	eMsgs, err := l.chatBiz.ListContextMessages(ctx, chatId)
	if err != nil {
		slog.ErrorContext(ctx, "list context messages failed",
			slog.String("chat_id", chatId.String()),
			slog.Any("err", err),
		)

		return nil, errors.WithMessage(err, "list context messages failed")
	}

	return eMsgs, nil
}

// 结束任务处理 写入最终结果 并落库
func (l *Logic) finalizingProcess(
	ctx context.Context,
	state *chatSessionState,
	answerMsg *einoschema.Message,
	processErr error,
	addMessageWhenErr bool,
	errorContent string,
) error {
	if processErr != nil {
		if errors.Is(processErr, context.Canceled) {
			// 任务是外部手动取消的 不需要处理
			slog.DebugContext(ctx,
				fmt.Sprintf("task is canceled, task_id=%s, chat_id=%s",
					state.taskId, state.chatId.String()),
			)
			return nil
		}

		slog.WarnContext(ctx, "chat agent loop failed",
			slog.String("chat_id", state.chatId.String()),
			slog.Any("err", processErr),
		)

		if errorContent == "" {
			errorContent = "生成失败，请重试"
		}

		defer func() {
			l.emitErrorFinishStreamEvent(ctx, state, errorContent, chatmodel.FinishReasonStop)
			l.cleanProcess(ctx, state)
		}()

		if addMessageWhenErr {
			_, err := l.chatBiz.AddAssistantSystemMessage(ctx,
				&bizchat.AddAssistantSystemMessageCommand{
					ChatId:  state.chatId,
					UserId:  state.userId,
					Content: errorContent,
				})
			if err != nil {
				slog.ErrorContext(ctx, "add assistant system message failed",
					slog.String("chat_id", state.chatId.String()),
					slog.Any("err", err),
				)
			}
		}
	}

	if answerMsg == nil {
		return nil
	}

	extra := buildMessageExtra(state.sourceDocs)

	// 拿出全部结果 整合成最终结果后落库
	// 最终的结果落库
	_, err := l.chatBiz.AddAssistantMessage(ctx,
		&bizchat.AddAssistantMessageCommand{
			ChatId:  state.chatId,
			UserId:  state.userId,
			Content: answerMsg.Content,
			ReasoningContent: &chatmodel.MessageReasoningContent{
				Content: answerMsg.ReasoningContent,
			},
			Extra: extra,
		})
	if err != nil {
		slog.ErrorContext(ctx,
			"add assistant message failed",
			slog.String("chat_id", state.chatId.String()),
			slog.Any("err", err),
		)
	}

	// 先写入最后一个block
	l.emitNormalFinishStreamEvent(
		ctx,
		state,
		chatmodel.FinishReason(answerMsg.ResponseMeta.FinishReason),
	)
	l.cleanProcess(ctx, state)

	return nil
}

func (l *Logic) cleanProcess(
	ctx context.Context,
	state *chatSessionState,
) {
	ttl := conf.Global().Logic.Chat.GetTaskTimeout()
	if err := l.eventManager.SetEventStreamTTL(ctx, state.taskId, ttl); err != nil {
		slog.ErrorContext(ctx, "set event stream ttl failed",
			slog.String("task_id", state.taskId),
			slog.Any("err", err),
		)
	}

	// 最后设置任务状态为finish
	if err := l.eventManager.UpdateTaskStatus(
		ctx,
		state.taskId,
		chatmodel.MessageStreamTaskStatusFinished,
		ttl,
	); err != nil {
		slog.ErrorContext(ctx, "update task status failed",
			slog.String("task_id", state.taskId),
			slog.Any("err", err),
		)
	}
}

func (l *Logic) buildChatAgent(
	ctx context.Context,
	chatId uuid.UUID,
	sessionState *chatSessionState,
) (*bizagent.Agent[*chatSessionState], error) {
	var (
		provider = conf.Global().Logic.Chat.ModelProvider
		model    = conf.Global().Logic.Chat.Model
	)

	chatLLM, err := l.llmGateway.GetProvider(provider)
	if err != nil {
		slog.ErrorContext(ctx, "get chat llm failed",
			slog.String("chat_id", chatId.String()),
			slog.String("model_provider", provider.String()),
			slog.Any("err", err),
		)
		return nil, errors.WithMessage(err, "get chat llm failed")
	}

	options := llmchat.BuildLLMOptions(llmchat.BuildThinkingOption(provider, sessionState.enableThinking))
	if model != "" {
		options = append(options, llmchat.BuildLLMModelOption(model))
	}

	agentConfig := bizagent.Config[*chatSessionState]{
		MaxRound:         conf.Global().Logic.Chat.GetMaxRound(),
		LLM:              chatLLM,
		Options:          options,
		BeforeChat:       l.agentBeforeChatHook,
		BeforeRound:      l.agentBeforeRoundHook,
		MsgAppender:      l.agentMessageAppender,
		OnReasoningDelta: l.agentOnReasoningHook,
		OnReasoningEnd:   l.agentOnReasoningEndHook,
		OnContentDelta:   l.agentOnContentHook,
	}

	agent := bizagent.New(agentConfig, sessionState)

	return agent, nil
}

// 每一轮的所有消息都写入上下文
func (l *Logic) agentMessageAppender(
	ctx context.Context,
	state *chatSessionState,
	newMsgs []*einoschema.Message,
) {
	if err := l.chatBiz.AppendContextMessage(ctx, state.chatId, newMsgs); err != nil {
		slog.ErrorContext(ctx, "append context message failed",
			slog.String("chat_id", state.chatId.String()),
			slog.Any("err", err),
		)
	}
}

func (l *Logic) agentOnReasoningHook(
	ctx context.Context,
	round int,
	state *chatSessionState,
	delta string,
) error {
	l.emitReasoningStreamEvent(ctx, state, delta, false)

	return nil
}

func (l *Logic) agentOnReasoningEndHook(
	ctx context.Context,
	round int,
	state *chatSessionState,
) error {
	l.emitReasoningStreamEvent(ctx, state, "", true)

	return nil
}

func (l *Logic) agentOnContentHook(
	ctx context.Context,
	round int,
	state *chatSessionState,
	delta string,
) error {
	l.emitAnswerStreamEvent(ctx, state, delta)

	return nil
}

func (l *Logic) agentBeforeChatHook(
	ctx context.Context,
	state *chatSessionState,
	msgs []*einoschema.Message) (
	[]*einoschema.Message, error,
) {
	chatTemplate := l.chatTemplateManager.Get(state.userLang)
	templateVars := buildChatTemplateVars(state)

	systemPrompt, err := chatTemplate.Message(ctx, templateVars)
	if err != nil {
		return nil, errors.WithMessage(err, "render chat prompt template failed")
	}

	newMsgs := make([]*einoschema.Message, 0, len(msgs)+1)
	newMsgs = append(newMsgs, systemPrompt)
	newMsgs = append(newMsgs, msgs...)

	return newMsgs, nil
}

func (l *Logic) agentBeforeRoundHook(
	ctx context.Context,
	round int,
	state *chatSessionState,
	msgs []*einoschema.Message,
) ([]*einoschema.Message, error) {
	// 如果只剩下最后一轮 要求模型马上输出最终结果
	if round >= conf.Global().Logic.Chat.GetMaxRound()-1 {
		// 注入一条msg
		msgs = append(msgs, &einoschema.Message{
			Role:    einoschema.User,
			Content: "这轮输出是你最后一轮输出，请直接输出最终结果，不需要再进行工具调用，按照你已有的信息输出最终结果",
		})
	}

	return msgs, nil
}

func (l *Logic) emitStreamEvent(
	ctx context.Context,
	state *chatSessionState,
	event *chatmodel.MessageStreamEvent,
) {
	task, err := l.eventManager.GetTask(ctx, state.taskId)
	if err != nil {
		if errors.Is(err, bizchat.ErrTaskNotFound) {
			return
		}

		slog.ErrorContext(ctx, "get stream task failed",
			slog.String("task_id", state.taskId),
			slog.Any("err", err),
		)
		return
	}

	if !task.Status.IsRunning() {
		if task.Status.IsAborted() { // lazy check task status
			state.taskAborted = true
			state.cancel() // 取消正在进行的流式输出
			return
		}
		return
	}

	_, err = l.eventManager.AppendEvent(ctx, task.Id, event)
	if err != nil {
		slog.ErrorContext(ctx, "append stream event failed",
			slog.String("chat_id", state.chatId.String()),
			slog.String("task_id", state.taskId),
			slog.Any("err", err),
		)
	}
}

func (l *Logic) emitRetrievingStreamEvent(
	ctx context.Context,
	state *chatSessionState,
	typing bool,
) {
	timestamp := time.Now().Unix()
	status := chatmodel.MessageStreamTyping
	if !typing {
		status = chatmodel.MessageStreamFinished
	}
	event := &chatmodel.MessageStreamEvent{
		Id:        state.nextId(),
		Timestamp: timestamp,
		Phase: &chatmodel.MessageStreamPhaseData{
			Type:   chatmodel.MessageStreamPhaseRetrieving,
			Status: status,
		},
	}

	event.Phase.Citation = buildPhaseCitation(state.sourceDocs)
	if event.Phase.Citation != nil {
		event.Phase.Status = chatmodel.MessageStreamFinished
	}

	l.emitStreamEvent(ctx, state, event)
}

func (l *Logic) emitReasoningStreamEvent(
	ctx context.Context,
	state *chatSessionState,
	reasoningContent string,
	end bool,
) {
	timestamp := time.Now().Unix()
	event := &chatmodel.MessageStreamEvent{
		Id:        state.nextId(),
		Timestamp: timestamp,
		Phase: &chatmodel.MessageStreamPhaseData{
			Type:   chatmodel.MessageStreamPhaseThinking,
			Status: chatmodel.MessageStreamTyping,
		},
	}
	if end {
		event.Phase.Status = chatmodel.MessageStreamFinished
	} else {
		event.Phase.Content = reasoningContent
	}

	l.emitStreamEvent(ctx, state, event)
}

func (l *Logic) emitAnswerStreamEvent(
	ctx context.Context,
	state *chatSessionState,
	content string,
) {
	timestamp := time.Now().Unix()
	event := &chatmodel.MessageStreamEvent{
		Id:        state.nextId(),
		Timestamp: timestamp,
		Phase: &chatmodel.MessageStreamPhaseData{
			Type:    chatmodel.MessageStreamPhaseAnswer,
			Status:  chatmodel.MessageStreamTyping,
			Content: content,
		},
	}

	l.emitStreamEvent(ctx, state, event)
}

// 流式输出完成 结束流式输出
func (l *Logic) emitNormalFinishStreamEvent(
	ctx context.Context,
	state *chatSessionState,
	finishReason chatmodel.FinishReason,
) {
	timestamp := time.Now().Unix()
	event := &chatmodel.MessageStreamEvent{
		Id:           state.nextId(),
		Timestamp:    timestamp,
		Finished:     true,
		FinishReason: finishReason,
		Phase: &chatmodel.MessageStreamPhaseData{
			Type:   chatmodel.MessageStreamPhaseAnswer,
			Status: chatmodel.MessageStreamFinished,
		},
	}

	l.emitStreamEvent(ctx, state, event)
}

// 流式输出中途出错 结束流式输出
func (l *Logic) emitErrorFinishStreamEvent(
	ctx context.Context,
	state *chatSessionState,
	content string,
	finishReason chatmodel.FinishReason,
) {
	timestamp := time.Now().Unix()
	event := &chatmodel.MessageStreamEvent{
		Id:           state.nextId(),
		Timestamp:    timestamp,
		Finished:     true,
		FinishReason: finishReason,
		Phase: &chatmodel.MessageStreamPhaseData{
			Type:    chatmodel.MessageStreamPhaseAnswer,
			Status:  chatmodel.MessageStreamFinished,
			Action:  chatmodel.MessageStreamPhaseContentActionOverride,
			Content: content,
		},
	}

	l.emitStreamEvent(ctx, state, event)
}
