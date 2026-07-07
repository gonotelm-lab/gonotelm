package chat

import (
	"context"
	"log/slog"
	"sync"
	"time"

	chatagent "github.com/gonotelm-lab/gonotelm/internal/application/chat/agent"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	domainagent "github.com/gonotelm-lab/gonotelm/internal/domain/agent"
	chatentity "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/service/agentize"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/idgen"
	"github.com/gonotelm-lab/gonotelm/pkg/safe"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type CreateMessageHandler struct {
	wg *sync.WaitGroup

	notebookRepo           notebookrepo.Repository
	chatRepo               chatrepo.Repository
	chatMessageRepo        chatrepo.MessageRepository
	chatContextMessageRepo chatrepo.ContextMessageRepository
	streamTaskRepo         chatrepo.StreamTaskRepository
	sourceRepo             sourcerepo.Repository
	sourceStorageRepo      sourcerepo.StorageRepository
	sourceDocRepo          sourcerepo.SourceDocRepository
	sourceAgentizeService  *agentize.Service
	gateway                *chat.Gateway

	agentService *agentize.Service
}

func NewCreateMessageHandler(
	wg *sync.WaitGroup,
	notebookRepo notebookrepo.Repository,
	chatRepo chatrepo.Repository,
	chatMessageRepo chatrepo.MessageRepository,
	chatContextMessageRepo chatrepo.ContextMessageRepository,
	streamTaskRepo chatrepo.StreamTaskRepository,
	sourceRepo sourcerepo.Repository,
	sourceStorageRepo sourcerepo.StorageRepository,
	sourceDocRepo sourcerepo.SourceDocRepository,
	gateway *chat.Gateway,
) *CreateMessageHandler {
	sourceAgentizeService := agentize.NewService(
		agentize.Config{},
		sourceRepo,
		sourceStorageRepo,
		sourceDocRepo,
	)
	return &CreateMessageHandler{
		wg:                     wg,
		notebookRepo:           notebookRepo,
		chatRepo:               chatRepo,
		chatMessageRepo:        chatMessageRepo,
		chatContextMessageRepo: chatContextMessageRepo,
		streamTaskRepo:         streamTaskRepo,
		sourceRepo:             sourceRepo,
		sourceStorageRepo:      sourceStorageRepo,
		sourceDocRepo:          sourceDocRepo,
		sourceAgentizeService:  sourceAgentizeService,
		gateway:                gateway,

		agentService: sourceAgentizeService,
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
	targetChat, err := h.chatRepo.FindById(ctx, cmd.ChatId)
	if err != nil {
		return nil, errors.WithMessage(err, "find chat failed")
	}

	targetSources, err := h.filterReadySources(ctx, targetChat.NotebookId, cmd.SourceIds)
	if err != nil {
		return nil, errors.WithMessagef(err,
			"failed to filter ready sources, chat_id=%s, source_ids=%v",
			cmd.ChatId, cmd.SourceIds,
		)
	}

	userId := pkgcontext.GetUserId(ctx)
	if targetChat.OwnerId != userId {
		return nil, errors.ErrParams.Msgf("chat not belong to user, chat_id=%s", cmd.ChatId)
	}

	targetNotebook, err := h.notebookRepo.FindById(ctx, targetChat.NotebookId)
	if err != nil {
		return nil, err
	}

	newCtx := context.WithoutCancel(ctx)
	taskCtx, taskCancel := context.WithCancel(newCtx)
	// 1. add user task
	task, eventChan := h.initStreamTask(taskCtx, taskCancel, cmd.ChatId, userId)
	err = h.streamTaskRepo.Save(ctx, task)
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
	}
	h.wg.Go(func() { h.beginStreamTask(taskCtx, cmd, bundle) })

	return &CreateMessageResult{
		MsgId:  userMsg.Id,
		TaskId: task.Id,
	}, nil
}

func (h *CreateMessageHandler) filterReadySources(
	ctx context.Context,
	notebookId valobj.Id,
	sourceIds []valobj.Id,
) ([]*sourceentity.Source, error) {
	if len(sourceIds) == 0 {
		return nil, nil
	}

	sources, err := h.sourceRepo.GetByNotebookIdAndIds(ctx, notebookId, sourceIds)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to get sources, notebook_id=%s, source_ids=%v", notebookId, sourceIds)
	}

	if len(sources) == 0 {
		return nil, errors.ErrParams.Msgf("no sources found, notebook_id=%s, source_ids=%v", notebookId, sourceIds)
	}

	return sources, nil
}

type streamTaskBundle struct {
	cancel context.CancelFunc

	taskId, msgId, notebookId, chatId valobj.Id

	userMsg        *chatentity.Message
	assistantMsg   *chatentity.Message
	targetNotebook *notebookentity.Notebook
	targetChat     *chatentity.Chat
	targetSources  []*sourceentity.Source

	eventChan chan *chatentity.StreamTaskEvent
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
			err = errors.ErrInner.Msgf("stream task panic: %v", p)
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
					Message: "系统错误，请稍后重试",
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
		bundle.cancel()
	}()

	agt := chatagent.New(h.sourceAgentizeService, h.gateway, h.sourceRepo, h.notebookRepo)
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
	chatCfg := conf.Global().Logic.Chat
	runResponse, err := agt.Run(ctx, &chatagent.RunRequest{
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
		task.Status = chatentity.StreamTaskStatusFinished
		if err := h.streamTaskRepo.Save(ctx, task); err != nil {
			slog.ErrorContext(ctx, "save finished stream task failed",
				slog.Any("task_id", taskId),
				slog.Any("err", err),
			)
		}
	}

	if err := h.streamTaskRepo.SetStreamTTL(ctx, taskId, 10*time.Minute*10); err != nil {
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
	userId string,
) (*chatentity.StreamTask, chan *chatentity.StreamTaskEvent) {
	task := chatentity.NewStreamTask(chatId, userId)
	eventChan := make(chan *chatentity.StreamTaskEvent, 1024)

	h.wg.Go(func() {
		safe.Do(ctx, func() error {
			h.consumeStreamTaskEvents(ctx, cancel, task.Id, eventChan)
			return nil
		})()
	})

	return task, eventChan
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
