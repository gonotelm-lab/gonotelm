package chat

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"runtime/debug"
	"sync"
	"time"

	bizchat "github.com/gonotelm-lab/gonotelm/internal/app/biz/chat"
	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	chatmodel "github.com/gonotelm-lab/gonotelm/internal/app/model/chat"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
)

type Logic struct {
	wg           sync.WaitGroup
	notebookBiz  *biznotebook.Biz
	sourceBiz    *bizsource.Biz
	chatBiz      *bizchat.Biz
	eventManager *bizchat.ChatEventManager

	llm einomodel.ToolCallingChatModel
}

func NewLogic(
	llm einomodel.ToolCallingChatModel,
	notebookBiz *biznotebook.Biz,
	sourceBiz *bizsource.Biz,
	chatBiz *bizchat.Biz,
	eventManager *bizchat.ChatEventManager,
) *Logic {
	return &Logic{
		notebookBiz:  notebookBiz,
		sourceBiz:    sourceBiz,
		chatBiz:      chatBiz,
		eventManager: eventManager,
		llm:          llm,
	}
}

func (l *Logic) Close(ctx context.Context) {
	l.wg.Wait()
	slog.InfoContext(ctx, "chat logic closed")
}

type CreateUserMessageParams struct {
	NotebookId uuid.UUID
	Prompt     string
	SourceIds  []uuid.UUID
}

type CreateUserMessageResult struct {
	MsgId  uuid.UUID
	TaskId string
}

func (l *Logic) CreateUserMessage(
	ctx context.Context,
	params *CreateUserMessageParams,
) (*CreateUserMessageResult, error) {
	// check notebook
	_, err := l.notebookBiz.GetNotebook(ctx, params.NotebookId)
	if err != nil {
		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
			return nil, errors.ErrParams.Msgf("notebook not found, notebook_id=%s", params.NotebookId)
		}
		return nil, errors.WithMessage(err, "get notebook failed")
	}

	// 粗略检查source ids是否存在且属于notebookid
	if len(params.SourceIds) > 0 {
		query := &bizsource.CheckSourceIdsQuery{
			NotebookId: params.NotebookId,
			SourceIds:  params.SourceIds,
		}
		existSourceIds, err := l.sourceBiz.CheckSourceIds(ctx, query)
		if err != nil {
			return nil, errors.WithMessage(err, "check source ids failed")
		}

		if len(existSourceIds) == 0 {
			return nil, errors.ErrParams.Msgf(
				"no source ids found, notebook_id=%s, source_ids=%v",
				params.NotebookId, params.SourceIds)
		}
	}

	// 先注册任务
	streamTask, err := l.eventManager.CreateTask(ctx, &bizchat.CreateTaskCommand{
		ChatId: params.NotebookId.String(),
		UserId: pkgcontext.GetUserId(ctx),
	})
	if err != nil {
		return nil, errors.WithMessage(err, "create task failed")
	}

	chatId := params.NotebookId
	userMsgId, err := l.chatBiz.AddUserMessage(ctx, &bizchat.AddUserMessageCommand{
		ChatId:  chatId,
		UserId:  pkgcontext.GetUserId(ctx),
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
	go l.processUserMessageTask(ctx, streamTask.Id, params.NotebookId, userMsgId, params)

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

type RetrieveSourceDocsParams struct {
	NotebookId uuid.UUID
	Prompt     string
	SourceIds  []uuid.UUID
}

func (l *Logic) RetrieveSourceDocs(
	ctx context.Context,
	params *RetrieveSourceDocsParams,
) ([]*model.SourceDoc, error) {
	query := &bizsource.CheckSourceIdsQuery{
		NotebookId: params.NotebookId,
		SourceIds:  params.SourceIds,
	}
	existSourceIds, err := l.sourceBiz.CheckSourceIds(ctx, query)
	if err != nil {
		return nil, errors.WithMessage(err, "check source ids failed")
	}

	if len(existSourceIds) == 0 {
		return nil, errors.ErrParams.Msgf(
			"no source ids found, notebook_id=%s, source_ids=%v",
			params.NotebookId, params.SourceIds)
	}

	retrieved, err := l.sourceBiz.RetrieveSourceDocs(ctx,
		&bizsource.RetrieveSourceDocsQuery{
			NotebookId: params.NotebookId,
			SourceIds:  existSourceIds,
			Query:      params.Prompt,
			Count:      conf.Global().Logic.Chat.GetSourceDocsRecallCount(),
		})
	if err != nil {
		return nil, errors.WithMessage(err, "recall source docs failed")
	}

	slog.DebugContext(ctx, fmt.Sprintf("successfully recalled %d source docs", len(retrieved)))

	return retrieved, nil
}

func (l *Logic) processUserMessageTask(
	ctx context.Context,
	taskId string,
	chatId uuid.UUID,
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

	if !l.processCheckUserMessage(ctx, chatId, msgId) {
		return
	}

	userId := pkgcontext.GetUserId(ctx)
	sessionState := &chatSessionState{
		id:     0, // accumulated id
		taskId: taskId,
		chatId: chatId,
		userId: userId,
	}
	ctx, sessionState.cancel = context.WithCancel(ctx)

	if len(params.SourceIds) > 0 {
		l.emitRetrievingStreamEvent(ctx, sessionState)
	}
	_, ok := l.processCheckSourceDocs(
		ctx,
		params.NotebookId,
		params.Prompt,
		params.SourceIds,
		taskId,
	)
	if !ok {
		return
	}

	contextMsgs, ok := l.processCheckGetMessageContext(ctx, chatId)
	if !ok {
		return
	}

	chatAgent := l.buildNewChatAgent(sessionState)
	answer, err := chatAgent.produceAnswer(ctx, chatId, contextMsgs)
	if err != nil {
		slog.ErrorContext(ctx, "chat agent loop failed",
			slog.String("chat_id", chatId.String()),
			slog.Any("err", err),
		)
	}

	if finalErr := l.finalizingProcess(
		ctx,
		sessionState,
		answer,
		err,
	); finalErr != nil {
		slog.ErrorContext(ctx, "finalizing process failed",
			slog.String("chat_id", chatId.String()),
			slog.Any("err", finalErr),
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

	sourceDocs, err := l.RetrieveSourceDocs(ctx,
		&RetrieveSourceDocsParams{
			NotebookId: notebookId,
			Prompt:     prompt,
			SourceIds:  sourceIds,
		})
	if err != nil {
		slog.ErrorContext(ctx, "chat logicrecall source docs failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Int("source_docs_count", len(sourceDocs)),
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
	finalErr error,
) error {
	if finalErr != nil {
		if errors.Is(finalErr, context.Canceled) {
			// 任务是手动取消的 不需要处理
			slog.DebugContext(ctx, fmt.Sprintf("task is canceled, task_id=%s, chat_id=%s", state.taskId, state.chatId.String()))
			return nil
		}

		_, err := l.chatBiz.AddAssistantSystemMessage(ctx,
			&bizchat.AddAssistantSystemMessageCommand{
				ChatId:  state.chatId,
				UserId:  state.userId,
				Content: "生成失败，请重试",
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
		})
	if err != nil {
		slog.ErrorContext(ctx,
			"add assistant message failed",
			slog.String("chat_id", state.chatId.String()),
			slog.Any("err", err),
		)
	}

	ttl := conf.Global().Logic.Chat.GetTaskTimeout()

	// 先写入最后一个block
	l.emitFinishStreamEvent(
		ctx,
		state,
		chatmodel.FinishReason(answerMsg.ResponseMeta.FinishReason),
	)

	if err = l.eventManager.SetEventStreamTTL(ctx, state.taskId, ttl); err != nil {
		slog.ErrorContext(ctx, "set event stream ttl failed",
			slog.String("task_id", state.taskId),
			slog.Any("err", err),
		)
	}

	// 最后设置任务状态为finish
	if err = l.eventManager.UpdateTaskStatus(
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

	return nil
}

func (l *Logic) buildNewChatAgent(sessionState *chatSessionState) *agent[*chatSessionState] {
	var (
		maxRound    = 10
		agentConfig = agentConfig[*chatSessionState]{
			maxRound:       maxRound,
			llm:            l.llm,
			beforeChat:     l.agentBeforeChatHook,
			beforeRound:    l.agentBeforeRoundHook,
			msgAppender:    l.agentMessageAppender,
			onReasoning:    l.agentOnReasoningHook,
			onReasoningEnd: l.agentOnReasoningEndHook,
			onContent:      l.agentOnContentHook,
		}
	)

	agent := newAgent(agentConfig, sessionState)

	// TODO bind tools if needed

	return agent
}

// 每一轮的所有消息都写入上下文
func (l *Logic) agentMessageAppender(
	ctx context.Context,
	chatId uuid.UUID,
	newMsgs []*einoschema.Message,
) {
	if err := l.chatBiz.AppendContextMessage(ctx, chatId, newMsgs); err != nil {
		slog.ErrorContext(ctx, "append context message failed",
			slog.String("chat_id", chatId.String()),
			slog.Any("err", err),
		)
	}
}

func (l *Logic) agentOnReasoningHook(
	ctx context.Context,
	round int,
	chatId uuid.UUID,
	msg *einoschema.Message,
	state *chatSessionState,
) error {
	l.emitReasoningStreamEvent(ctx, state, msg.ReasoningContent, false)

	return nil
}

func (l *Logic) agentOnReasoningEndHook(
	ctx context.Context,
	round int,
	chatId uuid.UUID,
	msg *einoschema.Message,
	state *chatSessionState,
) error {
	l.emitReasoningStreamEvent(ctx, state, "", true)

	return nil
}

func (l *Logic) agentOnContentHook(
	ctx context.Context,
	round int,
	chatId uuid.UUID,
	msg *einoschema.Message,
	state *chatSessionState,
) error {
	l.emitAnswerStreamEvent(ctx, state, msg.Content)

	return nil
}

func (l *Logic) agentBeforeChatHook(
	ctx context.Context,
	chatId uuid.UUID,
	msgs []*einoschema.Message) (
	[]*einoschema.Message, error,
) {
	// 注入system prompt
	systemPrompt := &einoschema.Message{
		Role:    einoschema.System,
		Content: `你是一个有用的助手，请根据用户的问题和上下文，给出详细的回答。`,
	}

	newMsgs := make([]*einoschema.Message, 0, len(msgs)+1)
	newMsgs = append(newMsgs, systemPrompt)
	newMsgs = append(newMsgs, msgs...)

	return newMsgs, nil
}

func (l *Logic) agentBeforeRoundHook(
	ctx context.Context,
	chatId uuid.UUID,
	round int,
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
func (l *Logic) emitFinishStreamEvent(
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
