package chat

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"runtime/debug"
	"sync"
	"time"

	bizagent "github.com/gonotelm-lab/gonotelm/internal/app/biz/agent"
	bizchat "github.com/gonotelm-lab/gonotelm/internal/app/biz/chat"
	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic/prompts"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	chatmodel "github.com/gonotelm-lab/gonotelm/internal/app/model/chat"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
)

const defaultPromptLang = "zh"

type Logic struct {
	wg           sync.WaitGroup
	notebookBiz  *biznotebook.Biz
	sourceBiz    *bizsource.Biz
	chatBiz      *bizchat.Biz
	eventManager *bizchat.ChatEventManager

	llmGateway          *gateway.Gateway
	chatTemplateManager *prompts.ChatTemplateManager
}

func MustNewLogic(
	llmGateway *gateway.Gateway,
	notebookBiz *biznotebook.Biz,
	sourceBiz *bizsource.Biz,
	chatBiz *bizchat.Biz,
	eventManager *bizchat.ChatEventManager,
) *Logic {
	chatTemplateManager, err := prompts.NewChatTemplateManager(defaultPromptLang)
	if err != nil {
		panic(err)
	}

	return &Logic{
		notebookBiz:         notebookBiz,
		sourceBiz:           sourceBiz,
		chatBiz:             chatBiz,
		eventManager:        eventManager,
		llmGateway:          llmGateway,
		chatTemplateManager: chatTemplateManager,
	}
}

func (l *Logic) Close(ctx context.Context) {
	l.wg.Wait()
	slog.InfoContext(ctx, "chat logic closed")
}

type CreateUserMessageParams struct {
	ChatId         uuid.UUID
	Prompt         string
	SourceIds      []uuid.UUID
	EnableThinking bool
}

type CreateUserMessageResult struct {
	MsgId  uuid.UUID
	TaskId string
}

func (l *Logic) CreateUserMessage(
	ctx context.Context,
	params *CreateUserMessageParams,
) (*CreateUserMessageResult, error) {
	targetChat, err := l.chatBiz.GetChat(ctx, params.ChatId)
	if err != nil {
		if errors.Is(err, bizchat.ErrChatNotFound) {
			return nil, errors.ErrParams.Msgf("chat not found, chat_id=%s", params.ChatId)
		}

		return nil, errors.WithMessage(err, "get chat failed")
	}

	userId := pkgcontext.GetUserId(ctx)

	if targetChat.OwnerId != userId {
		return nil, errors.ErrParams.Msgf("chat not belong to user, chat_id=%s, user_id=%s", params.ChatId, userId)
	}

	// check notebook
	_, err = l.notebookBiz.GetNotebook(ctx, targetChat.NotebookId)
	if err != nil {
		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
			return nil, errors.ErrParams.Msgf("notebook not found, notebook_id=%s", targetChat.NotebookId)
		}
		return nil, errors.WithMessage(err, "get notebook failed")
	}

	// 粗略检查source ids是否存在且属于notebookid
	if len(params.SourceIds) > 0 {
		query := &bizsource.CheckSourceIdsQuery{
			NotebookId: targetChat.NotebookId,
			SourceIds:  params.SourceIds,
		}
		existSourceIds, err := l.sourceBiz.CheckSourceIds(ctx, query)
		if err != nil {
			return nil, errors.WithMessage(err, "check source ids failed")
		}

		if len(existSourceIds) == 0 {
			return nil, errors.ErrParams.Msgf(
				"no source ids found, notebook_id=%s, source_ids=%v",
				targetChat.NotebookId, params.SourceIds)
		}
	}

	// 先注册任务
	streamTask, err := l.eventManager.CreateTask(ctx,
		&bizchat.CreateTaskCommand{
			ChatId: params.ChatId.String(),
			UserId: userId,
		})
	if err != nil {
		return nil, errors.WithMessage(err, "create task failed")
	}

	chatId := params.ChatId
	userMsgId, err := l.chatBiz.AddUserMessage(ctx, &bizchat.AddUserMessageCommand{
		ChatId:  chatId,
		UserId:  userId,
		Content: params.Prompt,
	})
	if err != nil {
		return nil, errors.WithMessage(err, "add user message failed")
	}

	einoMsg := &einoschema.Message{
		Role:    einoschema.User,
		Content: params.Prompt,
	}
	err = l.chatBiz.AppendContextMessage(ctx, chatId, []*einoschema.Message{einoMsg})
	if err != nil {
		return nil, errors.WithMessage(err, "append context message failed")
	}

	// 启动后台处理流程
	ctx = context.WithoutCancel(ctx)
	l.wg.Add(1)
	go l.processUserMessageTask(ctx, targetChat, streamTask.Id, userMsgId, params)

	return &CreateUserMessageResult{
		MsgId:  userMsgId,
		TaskId: streamTask.Id,
	}, nil
}

type ListMessagesParams struct {
	ChatId uuid.UUID
	Cursor int64
	Limit  int
}

type ListMessagesResult struct {
	Messages   []*chatmodel.Message
	HasMore    bool
	NextCursor int64
}

func (l *Logic) ListMessages(
	ctx context.Context,
	params *ListMessagesParams,
) (*ListMessagesResult, error) {
	cursor := params.Cursor
	if cursor == 0 {
		cursor = math.MaxInt64
	}
	if cursor < 0 {
		return nil, errors.ErrParams.Msgf("invalid cursor, cursor=%d", cursor)
	}

	fetchLimit := params.Limit + 1
	userId := pkgcontext.GetUserId(ctx)
	msgs, err := l.chatBiz.ListMessagesByCursor(
		ctx,
		&bizchat.ListMessagesByCursorQuery{
			ChatId: params.ChatId,
			UserId: userId,
			Cursor: cursor,
			Limit:  fetchLimit,
		},
	)
	if err != nil {
		return nil, errors.WithMessage(err, "list chat messages failed")
	}

	hasMore := len(msgs) > params.Limit
	if hasMore {
		msgs = msgs[:params.Limit]
	}

	nextCursor := int64(0)
	if hasMore && len(msgs) > 0 {
		nextCursor = msgs[len(msgs)-1].SeqNo
	}

	return &ListMessagesResult{
		Messages:   msgs,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// 清除会话的上下文缓存
func (l *Logic) ClearChatContext(
	ctx context.Context,
	chatId uuid.UUID,
) error {
	return nil
}

func (l *Logic) retrieveSourceDocs(
	ctx context.Context,
	notebookId uuid.UUID,
	prompt string,
	sourceIds []uuid.UUID,
	taskId string,
) ([]*model.SourceDoc, error) {
	query := &bizsource.CheckSourceIdsQuery{
		NotebookId: notebookId,
		SourceIds:  sourceIds,
	}
	existSourceIds, err := l.sourceBiz.CheckSourceIds(ctx, query)
	if err != nil {
		return nil, errors.WithMessage(err, "check source ids failed")
	}

	if len(existSourceIds) == 0 {
		return nil, errors.ErrParams.Msgf(
			"no source ids found, notebook_id=%s, source_ids=%v",
			notebookId, sourceIds)
	}

	retrieved, err := l.sourceBiz.RetrieveSourceDocs(ctx,
		&bizsource.RetrieveSourceDocsQuery{
			NotebookId: notebookId,
			SourceIds:  existSourceIds,
			Query:      prompt,
			Count:      conf.Global().Logic.Chat.GetSourceDocsRecallCount(),
		})
	if err != nil {
		return nil, errors.WithMessage(err, "recall source docs failed")
	}

	slog.DebugContext(ctx,
		fmt.Sprintf("successfully retrieved %d source docs", len(retrieved)),
		slog.String("task_id", taskId),
		slog.String("notebook_id", notebookId.String()),
	)

	return retrieved, nil
}

func (l *Logic) DeleteChatContext(
	ctx context.Context,
	chatId uuid.UUID,
) error {
	err := l.chatBiz.ClearChatContext(ctx, chatId)
	if err != nil {
		return errors.WithMessage(err, "clear chat context failed")
	}

	return nil
}

func (l *Logic) processUserMessageTask(
	ctx context.Context,
	chat *chatmodel.Chat,
	taskId string,
	msgId uuid.UUID,
	params *CreateUserMessageParams,
) {
	defer func() {
		l.wg.Done()

		if e := recover(); e != nil {
			stacks := debug.Stack()
			slog.ErrorContext(ctx, "background process user message panic",
				slog.Any("err", e),
				slog.String("stack", string(stacks)),
			)
		}
	}()

	if !l.processCheckUserMessage(ctx, chat.Id, msgId) {
		return
	}

	userId := pkgcontext.GetUserId(ctx)
	sessionState := &chatSessionState{
		id:             0, // accumulated id
		taskId:         taskId,
		chatId:         chat.Id,
		userId:         userId,
		enableThinking: params.EnableThinking,
	}
	ctx, sessionState.cancel = context.WithCancel(ctx)

	if len(params.SourceIds) > 0 {
		l.emitRetrievingStreamEvent(ctx, sessionState)
	}
	selectedSourcDocs, ok := l.processCheckSourceDocs(
		ctx,
		chat.NotebookId,
		params.Prompt,
		params.SourceIds,
		taskId,
	)
	if !ok {
		return
	}
	sessionState.sourceDocs = selectedSourcDocs
	if len(selectedSourcDocs) > 0 {
		l.emitRetrievingStreamEvent(ctx, sessionState)
	}

	contextMsgs, ok := l.processCheckGetMessageContext(ctx, chat.Id)
	if !ok {
		return
	}

	chatLLM, err := l.llmGateway.GetProvider(
		conf.Global().Logic.Chat.ModelProvider,
	)
	if err != nil {
		slog.ErrorContext(ctx, "get chat llm failed",
			slog.String("chat_id", chat.Id.String()),
			slog.String("model_provider", conf.Global().Logic.Chat.ModelProvider.String()),
			slog.Any("err", err),
		)
		return
	}

	chatAgent := l.buildNewChatAgent(chatLLM, sessionState)
	answer, err := chatAgent.Generate(ctx, contextMsgs)
	if err != nil {
		slog.ErrorContext(ctx, "chat agent loop failed",
			slog.String("chat_id", chat.Id.String()),
			slog.Any("err", err),
		)
	}

	if err := l.finalizingProcess(
		ctx,
		sessionState,
		answer,
		err,
	); err != nil {
		slog.ErrorContext(ctx, "finalizing process failed",
			slog.String("chat_id", chat.Id.String()),
			slog.Any("err", err),
		)
	}
}

// 检查目标消息是否存在
func (l *Logic) processCheckUserMessage(ctx context.Context, chatId, msgId uuid.UUID) bool {
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
		if _, err := l.chatBiz.AddAssistantSystemMessage(ctx,
			&bizchat.AddAssistantSystemMessageCommand{
				ChatId:  chatId,
				UserId:  pkgcontext.GetUserId(ctx),
				Content: "I'm sorry, I'm not able to process your request.",
			}); err != nil {
			slog.ErrorContext(ctx, "add assistant system message failed", logAttrs(err)...)
		}
		return false
	}

	if targetMsg.MsgRole != chatmodel.MessageRoleUser {
		slog.WarnContext(ctx, "target message is not a user message", logAttrs(nil)...)
		return false
	}

	return true
}

func (l *Logic) processCheckSourceDocs(
	ctx context.Context,
	notebookId uuid.UUID,
	prompt string,
	sourceIds []uuid.UUID,
	taskId string,
) ([]*model.SourceDoc, bool) {
	if len(sourceIds) == 0 {
		return nil, true
	}

	if l.isTaskAborted(ctx, taskId) {
		return nil, false
	}

	sourceDocs, err := l.retrieveSourceDocs(ctx, notebookId, prompt, sourceIds, taskId)
	if err != nil {
		slog.ErrorContext(ctx, "chat logic retrieve source docs failed",
			slog.String("task_id", taskId),
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)

		return nil, false
	}

	return sourceDocs, true
}

func (l *Logic) processCheckGetMessageContext(
	ctx context.Context,
	chatId uuid.UUID,
) ([]*einoschema.Message, bool) {
	eMsgs, err := l.chatBiz.ListContextMessages(ctx, chatId)
	if err != nil {
		slog.ErrorContext(ctx, "list context messages failed",
			slog.String("chat_id", chatId.String()),
			slog.Any("err", err),
		)

		return nil, false
	}

	return eMsgs, true
}

func (l *Logic) finalizingProcess(
	ctx context.Context,
	state *chatSessionState,
	answerMsg *einoschema.Message,
	answerErr error,
) error {
	if answerErr != nil {
		if errors.Is(answerErr, context.Canceled) {
			// 任务是手动取消的 不需要处理
			slog.DebugContext(ctx,
				fmt.Sprintf("task is canceled, task_id=%s, chat_id=%s",
					state.taskId, state.chatId.String()),
			)
			return nil
		}

		slog.WarnContext(ctx, "chat agent loop failed",
			slog.String("chat_id", state.chatId.String()),
			slog.Any("err", answerErr),
		)

		const errorContent = "生成失败，请重试"

		defer func() {
			l.emitErrorFinishStreamEvent(ctx, state, errorContent, chatmodel.FinishReasonStop)
			l.finalizingProcessClean(ctx, state)
		}()

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

	if answerMsg == nil {
		return nil
	}

	extra := buildMessageExtraFromSourceDocs(state.sourceDocs)

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
	l.finalizingProcessClean(ctx, state)

	return nil
}

func (l *Logic) finalizingProcessClean(
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

func (l *Logic) buildNewChatAgent(
	chatLLM einomodel.ToolCallingChatModel,
	sessionState *chatSessionState,
) *bizagent.Agent[*chatSessionState] {
	var (
		maxRound = 10
		options  = llmchat.BuildThinkingOptions(
			conf.Global().Logic.Chat.ModelProvider,
			sessionState.enableThinking,
		)

		agentConfig = bizagent.AgentConfig[*chatSessionState]{
			MaxRound:       maxRound,
			LLM:            chatLLM,
			Options:        options,
			BeforeChat:     l.agentBeforeChatHook,
			BeforeRound:    l.agentBeforeRoundHook,
			MsgAppender:    l.agentMessageAppender,
			OnReasoning:    l.agentOnReasoningHook,
			OnReasoningEnd: l.agentOnReasoningEndHook,
			OnContent:      l.agentOnContentHook,
		}
	)

	agent := bizagent.NewAgent(agentConfig, sessionState)

	// TODO bind tools if needed

	return agent
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
	msg *einoschema.Message,
	state *chatSessionState,
) error {
	l.emitReasoningStreamEvent(ctx, state, msg.ReasoningContent, false)

	return nil
}

func (l *Logic) agentOnReasoningEndHook(
	ctx context.Context,
	round int,
	msg *einoschema.Message,
	state *chatSessionState,
) error {
	l.emitReasoningStreamEvent(ctx, state, "", true)

	return nil
}

func (l *Logic) agentOnContentHook(
	ctx context.Context,
	round int,
	msg *einoschema.Message,
	state *chatSessionState,
) error {
	l.emitAnswerStreamEvent(ctx, state, msg.Content)

	return nil
}

func (l *Logic) agentBeforeChatHook(
	ctx context.Context,
	state *chatSessionState,
	msgs []*einoschema.Message) (
	[]*einoschema.Message, error,
) {
	chatTemplate := l.chatTemplateManager.Get(state.userLang)
	templateVars := buildChatTemplateVars(state.sourceDocs)

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
	// TODO 如果只剩下最后一轮 要求模型马上输出最终结果

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
) {
	timestamp := time.Now().Unix()
	event := &chatmodel.MessageStreamEvent{
		Id:        state.nextId(),
		Timestamp: timestamp,
		Phase: &chatmodel.MessageStreamPhaseData{
			Type:   chatmodel.MessageStreamPhaseRetrieving,
			Status: chatmodel.MessageStreamTyping,
		},
	}

	event.Phase.Citation = buildPhaseCitationFromSourceDocs(state.sourceDocs)
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

type groupedSourceDocCitation struct {
	SourceId string
	DocIds   []string
}

func groupSourceDocsForCitation(sourceDocs []*model.SourceDoc) []*groupedSourceDocCitation {
	if len(sourceDocs) == 0 {
		return nil
	}

	groups := make([]*groupedSourceDocCitation, 0, len(sourceDocs))
	for _, sourceDoc := range sourceDocs {
		if sourceDoc == nil {
			continue
		}

		sourceID := sourceDoc.SourceId.String()
		groupIdx := -1
		for idx, group := range groups {
			if group.SourceId == sourceID {
				groupIdx = idx
				break
			}
		}
		if groupIdx < 0 {
			groups = append(groups, &groupedSourceDocCitation{
				SourceId: sourceID,
				DocIds:   make([]string, 0, 1),
			})
			groupIdx = len(groups) - 1
		}

		if sourceDoc.Id != "" {
			groups[groupIdx].DocIds = append(groups[groupIdx].DocIds, sourceDoc.Id)
		}
	}

	return groups
}

func buildMessageExtraFromSourceDocs(sourceDocs []*model.SourceDoc) *chatmodel.MessageExtra {
	groupedSourceDocs := groupSourceDocsForCitation(sourceDocs)
	if len(groupedSourceDocs) == 0 {
		return nil
	}

	citations := make([]*chatmodel.Citation, 0, len(groupedSourceDocs))
	for _, grouped := range groupedSourceDocs {
		citations = append(citations, &chatmodel.Citation{
			SourceId: grouped.SourceId,
			DocIds:   grouped.DocIds,
		})
	}

	return &chatmodel.MessageExtra{
		Citation: citations,
	}
}

func buildPhaseCitationFromSourceDocs(sourceDocs []*model.SourceDoc) []*chatmodel.PhaseCitationItem {
	groupedSourceDocs := groupSourceDocsForCitation(sourceDocs)
	if len(groupedSourceDocs) == 0 {
		return nil
	}

	items := make([]*chatmodel.PhaseCitationItem, 0, len(groupedSourceDocs))
	for _, grouped := range groupedSourceDocs {
		item := &chatmodel.PhaseCitationItem{
			SourceId: grouped.SourceId,
			Docs:     make([]*chatmodel.PhaseCitationDoc, 0, len(grouped.DocIds)),
		}
		for _, docID := range grouped.DocIds {
			item.Docs = append(item.Docs, &chatmodel.PhaseCitationDoc{
				Id: docID,
				Position: &chatmodel.PhaseCitationDocPosition{
					// TODO 等文档记录了位置信息后这里需要重新填入
					Start: 0,
					End:   0,
				},
			})
		}
		items = append(items, item)
	}

	return items
}

func buildChatTemplateVars(sourceDocs []*model.SourceDoc) prompts.ChatTemplateVars {
	templateVars := prompts.ChatTemplateVars{}
	for _, sourceDoc := range sourceDocs {
		if sourceDoc == nil {
			continue
		}

		sourceID := sourceDoc.SourceId.String()
		groupIdx := -1
		for idx, group := range templateVars.SelectedSources {
			if group.SourceID == sourceID {
				groupIdx = idx
				break
			}
		}
		if groupIdx < 0 {
			templateVars.SelectedSources = append(templateVars.SelectedSources, prompts.ChatSelectedSourceGroup{
				SourceIndex: int64(len(templateVars.SelectedSources)),
				SourceID:    sourceID,
			})
			groupIdx = len(templateVars.SelectedSources) - 1
		}
		docIndex := int64(len(templateVars.SelectedSources[groupIdx].Docs))

		templateVars.SelectedSources[groupIdx].Docs = append(
			templateVars.SelectedSources[groupIdx].Docs,
			prompts.ChatSelectedSourceDoc{
				DocIndex: docIndex,
				DocID:    sourceDoc.Id,
				Content:  sourceDoc.Content,
				Score:    sourceDoc.Score,
			},
		)
	}

	return templateVars
}
