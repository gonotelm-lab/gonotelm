package chat

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"

	chatagent "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/agent"
	"github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/shared"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatentity "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chaterrors "github.com/gonotelm-lab/gonotelm/internal/domain/chat/errors"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/agentize"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	domainagent "github.com/gonotelm-lab/gonotelm/pkg/agent"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/idgen"
	"github.com/gonotelm-lab/gonotelm/pkg/safe"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func streamTaskCreateLockKey(userId valobj.Uid, chatId valobj.Id) string {
	return fmt.Sprintf("gonotelm:stream:task:lock:user:%s:chat:%s", userId, chatId)
}

type CreateMessageHandler struct {
	*baseHandler
	wg *sync.WaitGroup

	distLock               adapter.DistributedLock
	notebookRepo           notebookrepo.Repository
	chatMessageRepo        chatrepo.MessageRepository
	chatContextMessageRepo chatrepo.ContextMessageRepository
	streamTaskRepo         chatrepo.StreamTaskRepository
	sourceRepo             sourcerepo.Repository
	sourceStorageRepo      sourcerepo.StorageRepository
	sourceDocRepo          sourcerepo.SourceDocRepository
	sourceAgentizeService  *agentize.Service
	chatGateway            *llmchat.Gateway
	eventBus               eventbus.Publisher

	agentService *agentize.Service
}

func NewCreateMessageHandler(
	wg *sync.WaitGroup,
	distLock adapter.DistributedLock,
	notebookRepo notebookrepo.Repository,
	chatRepo chatrepo.ChatRepository,
	chatMessageRepo chatrepo.MessageRepository,
	chatContextMessageRepo chatrepo.ContextMessageRepository,
	streamTaskRepo chatrepo.StreamTaskRepository,
	sourceRepo sourcerepo.Repository,
	sourceStorageRepo sourcerepo.StorageRepository,
	sourceDocRepo sourcerepo.SourceDocRepository,
	chatGateway *llmchat.Gateway,
	eventBus eventbus.Publisher,
) *CreateMessageHandler {
	sourceAgentizeService := agentize.NewService(
		agentize.Config{},
		sourceRepo,
		sourceStorageRepo,
		sourceDocRepo,
	)
	return &CreateMessageHandler{
		baseHandler:            newBaseHandler(chatRepo),
		wg:                     wg,
		distLock:               distLock,
		notebookRepo:           notebookRepo,
		chatMessageRepo:        chatMessageRepo,
		chatContextMessageRepo: chatContextMessageRepo,
		streamTaskRepo:         streamTaskRepo,
		sourceRepo:             sourceRepo,
		sourceStorageRepo:      sourceStorageRepo,
		sourceDocRepo:          sourceDocRepo,
		sourceAgentizeService:  sourceAgentizeService,
		chatGateway:            chatGateway,
		eventBus:               eventBus,
		agentService:           sourceAgentizeService,
	}
}

type CreateMessageCommand struct {
	ChatId         valobj.Id
	Prompt         string
	SourceIds      []valobj.Id
	Style          chatagent.ChatMessageStyle
	AnswerLength   chatagent.ChatMessageAnswerLength
	EnableThinking bool
}

type CreateMessageResult struct {
	MsgId  valobj.Id
	TaskId valobj.Id
}

func (h *CreateMessageHandler) Handle(
	ctx context.Context,
	cmd *CreateMessageCommand,
) (*CreateMessageResult, error) {
	targetChat, err := h.commonHandle(ctx, cmd.ChatId)
	if err != nil {
		return nil, err
	}

	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.ChatScene)
	userId := pkgcontext.GetUserId(ctx)
	targetSources, err := shared.FilterReadySources(ctx, h.sourceRepo, targetChat.NotebookId, cmd.SourceIds, userId)
	if err != nil {
		return nil, errors.WithMessagef(err,
			"failed to filter ready sources, chat_id=%s, source_ids=%v",
			cmd.ChatId, cmd.SourceIds,
		)
	}

	targetNotebook, err := h.notebookRepo.FindById(ctx, targetChat.NotebookId)
	if err != nil {
		return nil, err
	}

	newCtx := context.WithoutCancel(ctx)
	taskCtx, taskCancel := context.WithCancel(newCtx)

	// make sure only one stream task is created for the same user+chat
	lockKey := streamTaskCreateLockKey(userId, cmd.ChatId)
	if err := h.distLock.Lock(ctx, lockKey); err != nil {
		taskCancel()
		return nil, errors.WithMessagef(err, "failed to lock stream task create, chat_id=%s", cmd.ChatId)
	}

	// 1. add user task
	task, eventChan, consumerDone, err := h.initStreamTask(
		taskCtx,
		taskCancel,
		cmd.ChatId,
		cmd.SourceIds,
		userId,
	)
	if err != nil {
		_ = h.distLock.Unlock(ctx, lockKey)
		taskCancel()
		return nil, errors.WithMessagef(err, "failed to init stream task, chat_id=%s", cmd.ChatId)
	}
	err = h.streamTaskRepo.Save(ctx, task)
	_ = h.distLock.Unlock(ctx, lockKey)
	if err != nil {
		taskCancel()
		return nil, errors.WithMessagef(err, "failed to save stream task, chat_id=%s", cmd.ChatId)
	}

	// 2. add user message
	userMsg := chatentity.NewUserTextMessage(cmd.ChatId, task.Id, userId, cmd.Prompt)
	err = h.chatMessageRepo.Save(ctx, userMsg)
	if err != nil {
		taskCancel()
		return nil, errors.WithMessagef(err, "failed to save user message, chat_id=%s", cmd.ChatId)
	}

	// 3. append context message
	ctxMsg := chatentity.NewUserContextMessage(cmd.ChatId, cmd.Prompt)
	err = h.chatContextMessageRepo.Append(ctx, cmd.ChatId, []*chatentity.ContextMessage{ctxMsg})
	if err != nil {
		taskCancel()
		return nil, errors.WithMessagef(err, "failed to append context message, chat_id=%s", cmd.ChatId)
	}

	bundle := &streamTaskBundle{
		cancel:         taskCancel,
		taskId:         task.Id,
		msgId:          userMsg.Id,
		notebookId:     targetNotebook.Id,
		chatId:         targetChat.Id,
		userMsg:        userMsg,
		assistantMsg:   chatentity.NewAssistantMessage(cmd.ChatId, task.Id, userId),
		targetNotebook: targetNotebook,
		targetChat:     targetChat,
		targetSources:  targetSources,
		eventChan:      eventChan,
		consumerDone:   consumerDone,
	}
	// assistant msg id as groupId
	taskCtx = pkgcontext.WithSceneGroupId(taskCtx, bundle.assistantMsg.Id.String())
	h.wg.Go(func() { h.beginStreamTask(taskCtx, cmd, bundle) })

	return &CreateMessageResult{
		MsgId:  userMsg.Id,
		TaskId: task.Id,
	}, nil
}

type streamTaskBundle struct {
	cancel context.CancelFunc

	taskId, msgId, notebookId, chatId valobj.Id

	userMsg        *chatentity.Message
	assistantMsg   *chatentity.Message
	targetNotebook *notebookentity.Notebook
	targetChat     *chatentity.Chat
	targetSources  []*sourceentity.Source

	eventChan    chan *chatentity.StreamTaskEvent
	consumerDone <-chan struct{}
}

func (b *streamTaskBundle) consumeEvents() {
	events := b.assistantMsg.ConsumeEvents()
	if len(events) == 0 {
		return
	}

	for _, evt := range events {
		if evt != nil {
			b.eventChan <- evt
		}
	}
}

// 开始处理流式消息
func (h *CreateMessageHandler) beginStreamTask(
	ctx context.Context,
	cmd *CreateMessageCommand,
	bundle *streamTaskBundle,
) {
	var (
		taskId = bundle.taskId
		msgId  = bundle.msgId
		err    error
	)

	defer func() {
		if p := recover(); p != nil {
			stacks := debug.Stack()
			err = errors.ErrInner.Msgf("stream task panic: %v, stack: %s", p, stacks)
		}

		if err != nil {
			slog.ErrorContext(ctx, "stream task failed",
				slog.Any("task_id", taskId),
				slog.Any("msg_id", msgId),
				slog.Any("err", err),
			)

			// abort stream task with error
			bundle.eventChan <- &chatentity.StreamTaskEvent{
				Id:         idgen.Get(taskId.String()),
				CreateTime: valobj.NewTime().Value(),
				Error: &chatentity.EventError{
					Message: "SystemError",
				},
			}
		} else {
			bundle.eventChan <- &chatentity.StreamTaskEvent{
				Id:         idgen.Get(taskId.String()),
				CreateTime: valobj.NewTime().Value(),
				Done:       true,
			}
		}

		h.finishStreamTask(ctx, taskId)
		close(bundle.eventChan)
		<-bundle.consumerDone // drain Done/Error before cancel so emit still has a live ctx
		bundle.cancel()
	}()

	agt := chatagent.New(h.sourceAgentizeService, h.chatGateway, h.sourceRepo, h.notebookRepo)
	slog.Info("start stream task", slog.Any("task_id", taskId), slog.Any("msg_id", msgId))

	// push assistant INIT event before agent run so clients can bind message id early
	bundle.consumeEvents()

	// get user history messages
	userMsgs, err := h.chatContextMessageRepo.ListAll(ctx, bundle.chatId)
	if err != nil {
		err = errors.WithMessagef(err, "failed to get user history messages, chat_id=%s", bundle.chatId)
		return
	}

	slog.DebugContext(ctx, "begin agent run",
		slog.Any("chat_id", bundle.chatId), slog.Any("task_id", taskId), slog.Any("msg_id", msgId),
	)
	chatCfg := conf.NotelmGlobal().Chat
	// block here
	runResponse, err := agt.RunV2(ctx,
		&chatagent.RunRequest{
			UserId:          pkgcontext.GetUserId(ctx),
			Notebook:        bundle.targetNotebook,
			Chat:            bundle.targetChat,
			Sources:         bundle.targetSources,
			ContextMessages: userMsgs,
			Style:           cmd.Style,
			AnswerLength:    cmd.AnswerLength,
			EnableThinking:  cmd.EnableThinking,
			Model:           chatCfg.Model,
			ModelProvider:   chatCfg.ModelProvider.String(),
			Hooks: chatagent.Hooks{
				RoundFinishedHook: h.onAgentRoundFinished(bundle),
				ThinkStart:        h.onAgentThinkStart(bundle),
				ThinkEnd:          h.onAgentThinkEnd(bundle),
				ThinkDelta:        h.onAgentThinkDelta(bundle),
				ResponseStart:     h.onAgentResponseStart(bundle),
				ResponseEnd:       h.onAgentResponseEnd(bundle),
				ResponseDelta:     h.onAgentResponseDelta(bundle),
				PhaseMarkHook:     h.onAgentMarkPhase(bundle),
			},
		})
	if err != nil {
		err = errors.WithMessagef(err, "failed to run agent, chat_id=%s", bundle.chatId)
		return
	}

	citations, citeErr := h.resolveMessageCitations(ctx, bundle.targetNotebook.Id, runResponse.SourceDocCitations)
	if citeErr != nil {
		slog.ErrorContext(ctx, "failed to resolve message citations",
			slog.Any("chat_id", bundle.chatId),
			slog.Any("err", citeErr),
		)
	} else {
		bundle.assistantMsg.SetCitations(citations)
	}
	bundle.consumeEvents() // push citations to event channel

	slog.DebugContext(ctx, "agent run done, now saving final assistant message",
		slog.Any("chat_id", bundle.chatId), slog.Any("task_id", taskId), slog.Any("msg_id", msgId),
	)
	// save final assistant message
	if err := h.chatMessageRepo.Save(ctx, bundle.assistantMsg); err != nil {
		slog.ErrorContext(ctx, "failed to save final assistant message",
			slog.Any("chat_id", bundle.chatId),
			slog.Any("err", err),
		)
	}

	slog.Info("stream task finished", slog.Any("task_id", taskId), slog.Any("msg_id", msgId))
}

func (h *CreateMessageHandler) finishStreamTask(ctx context.Context, taskId valobj.Id) {
	task, err := h.streamTaskRepo.FindById(ctx, taskId)
	if err != nil {
		slog.ErrorContext(ctx, "find stream task for finish failed",
			slog.Any("task_id", taskId),
			slog.Any("err", err),
		)
		return
	}

	if task.Status.IsRunning() {
		task.Finish()
		if err := h.streamTaskRepo.Save(ctx, task); err != nil {
			slog.ErrorContext(ctx, "save finished stream task failed",
				slog.Any("task_id", taskId),
				slog.Any("err", err),
			)
		} else {
			publishStreamTaskDomainEvents(ctx, h.eventBus, task)
		}
	}

	if err := h.streamTaskRepo.SetStreamTTL(ctx, taskId, chatentity.TaskExpireDuration); err != nil {
		slog.ErrorContext(ctx, "set stream task ttl failed",
			slog.Any("task_id", taskId),
			slog.Any("err", err),
		)
	}
}

func (h *CreateMessageHandler) onAgentRoundFinished(bundle *streamTaskBundle) chatagent.RoundFinishedHook {
	return func(ctx context.Context, newMsgs []*domainagent.EinoMessage) {
		msgs := make([]*chatentity.ContextMessage, 0, len(newMsgs))
		for _, msg := range newMsgs {
			msgs = append(msgs, &chatentity.ContextMessage{
				Id:         valobj.NewUnOrderedId().String(),
				CreateTime: valobj.NewTime().Value(),
				Message:    msg,
			})
		}

		if err := h.chatContextMessageRepo.Append(ctx, bundle.chatId, msgs); err != nil {
			slog.ErrorContext(ctx, "failed to append context messages",
				slog.Any("chat_id", bundle.chatId),
				slog.Any("err", err),
			)
		}
	}
}

func (h *CreateMessageHandler) onAgentThinkStart(bundle *streamTaskBundle) chatagent.ThinkStartHook {
	return func(ctx context.Context) {
		// create a new THINK fragment
		bundle.assistantMsg.BeginThinkFragment()
		bundle.consumeEvents()
	}
}

func (h *CreateMessageHandler) onAgentThinkEnd(bundle *streamTaskBundle) chatagent.ThinkEndHook {
	return func(ctx context.Context) {
		// end the current THINK fragment
		bundle.assistantMsg.EndThinkFragment()
		bundle.consumeEvents()
	}
}

func (h *CreateMessageHandler) onAgentThinkDelta(bundle *streamTaskBundle) chatagent.ThinkDeltaHook {
	return func(ctx context.Context, content string) {
		// append thinking content into the current THINK fragment
		bundle.assistantMsg.AppendThinkFragment(content)
		bundle.consumeEvents()
	}
}

func (h *CreateMessageHandler) onAgentResponseStart(bundle *streamTaskBundle) chatagent.ResponseStartHook {
	return func(ctx context.Context) {
		// begin a new RESPONSE fragment
		bundle.assistantMsg.BeginResponseFragment()
		bundle.consumeEvents()
	}
}

func (h *CreateMessageHandler) onAgentResponseEnd(bundle *streamTaskBundle) chatagent.ResponseEndHook {
	return func(ctx context.Context) {
		// end the current RESPONSE fragment
		bundle.assistantMsg.EndResponseFragment()
		bundle.consumeEvents()
	}
}

func (h *CreateMessageHandler) onAgentResponseDelta(bundle *streamTaskBundle) chatagent.ResponseDeltaHook {
	return func(ctx context.Context, delta string) {
		// append response delta into the current RESPONSE fragment
		bundle.assistantMsg.AppendResponseFragment(delta)
		bundle.consumeEvents()
	}
}

func (h *CreateMessageHandler) onAgentMarkPhase(bundle *streamTaskBundle) chatagent.PhaseMarkHook {
	return func(ctx context.Context, phase chatagent.Phase) {
		// create a new PHASE fragment
		bundle.assistantMsg.BeginPhaseFragment(phase.Summary, phase.Description)
		bundle.consumeEvents()
	}
}

func (h *CreateMessageHandler) initStreamTask(
	ctx context.Context,
	cancel context.CancelFunc,
	chatId valobj.Id,
	sourceIds []valobj.Id,
	userId valobj.Uid,
) (*chatentity.StreamTask, chan *chatentity.StreamTaskEvent, <-chan struct{}, error) {
	// first we check if there is a running task for this chat and user
	runningTask, err := h.streamTaskRepo.FindByUserAndChat(ctx, userId, chatId)
	if err != nil {
		if !errors.Is(err, chaterrors.ErrStreamTaskNotFound) {
			return nil, nil, nil, errors.WithMessagef(err, "failed to find running stream task, chat_id=%s", chatId)
		}
	}

	if runningTask != nil && runningTask.Status.IsRunning() {
		return nil, nil, nil, errors.ErrParams.Msgf("running stream task already exists, chat_id=%s", chatId)
	}

	const streamTaskBufferSize = 1024

	newTask := chatentity.NewStreamTask(chatId, sourceIds, userId)
	eventChan := make(chan *chatentity.StreamTaskEvent, streamTaskBufferSize)
	consumerDone := make(chan struct{})

	h.wg.Go(func() {
		defer close(consumerDone)
		safe.Do(ctx, func() error {
			h.consumeStreamTaskEvents(ctx, cancel, newTask.Id, eventChan)
			return nil
		})()
	})

	return newTask, eventChan, consumerDone, nil
}

func (h *CreateMessageHandler) consumeStreamTaskEvents(
	ctx context.Context,
	cancel context.CancelFunc,
	taskId valobj.Id,
	ch <-chan *chatentity.StreamTaskEvent,
) {
	for event := range ch {
		if ctx.Err() != nil {
			break
		}

		// check if task is already aborted
		targetTask, err := h.streamTaskRepo.FindById(ctx, taskId)
		if err != nil {
			slog.ErrorContext(ctx, "failed to find stream task",
				slog.Any("task_id", taskId),
				slog.Any("err", err),
			)
		}
		if targetTask != nil && targetTask.Status.IsAborted() {
			cancel()
			return
		}

		if err := h.streamTaskRepo.EmitStreamEvent(ctx, taskId, event); err != nil {
			slog.ErrorContext(ctx, "failed to emit stream task event",
				slog.Any("task_id", taskId),
				slog.Any("event_id", event.Id),
				slog.Any("err", err),
			)
		}
	}

	slog.Info("consume stream task events done", slog.Any("err", ctx.Err()))
}

func (h *CreateMessageHandler) resolveMessageCitations(
	ctx context.Context,
	notebookId valobj.Id,
	docIds []valobj.Id,
) ([]chatentity.MessageCitation, error) {
	if len(docIds) == 0 {
		return nil, nil
	}

	docs, err := h.sourceDocRepo.BatchFind(ctx, notebookId, uuid.EmptyUUID(), docIds)
	if err != nil {
		return nil, errors.WithMessage(err, "batch find source docs for citations failed")
	}

	docByID := make(map[string]*sourceentity.SourceDoc, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		docByID[doc.Id.String()] = doc
	}

	citations := make([]chatentity.MessageCitation, 0, len(docIds))
	for _, docId := range docIds {
		doc, ok := docByID[docId.String()]
		if !ok {
			return nil, errors.ErrParams.Msgf("source doc not found, doc_id=%s", docId)
		}
		citations = append(citations, chatentity.MessageCitation{
			DocId:    doc.Id,
			SourceId: doc.SourceId,
		})
	}

	return citations, nil
}
