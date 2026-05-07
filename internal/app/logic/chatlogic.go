package logic

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"

	bizchat "github.com/gonotelm-lab/gonotelm/internal/app/biz/chat"
	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
)

type ChatLogic struct {
	wg          sync.WaitGroup
	notebookBiz *biznotebook.Biz
	sourceBiz   *bizsource.Biz
	chatBiz     *bizchat.Biz

	llm einomodel.ToolCallingChatModel
}

func NewChatLogic(
	llm einomodel.ToolCallingChatModel,
	notebookBiz *biznotebook.Biz,
	sourceBiz *bizsource.Biz,
	chatBiz *bizchat.Biz,
) *ChatLogic {
	return &ChatLogic{
		notebookBiz: notebookBiz,
		sourceBiz:   sourceBiz,
		chatBiz:     chatBiz,
		llm:         llm,
	}
}

func (l *ChatLogic) Close(ctx context.Context) {
	l.wg.Wait()
	slog.InfoContext(ctx, "chat logic closed")
}

type CreateUserMessageParams struct {
	NotebookId uuid.UUID
	Prompt     string
	SourceIds  []uuid.UUID
}

func (l *ChatLogic) CreateUserMessage(
	ctx context.Context,
	params *CreateUserMessageParams,
) (uuid.UUID, error) {
	emptyId := uuid.UUID{}
	// check notebook
	_, err := l.notebookBiz.GetNotebook(ctx, params.NotebookId)
	if err != nil {
		if errors.Is(err, biznotebook.ErrNotebookNotFound) {
			return emptyId, errors.ErrParams.Msgf("notebook not found, notebook_id=%s", params.NotebookId)
		}
		return emptyId, errors.WithMessage(err, "get notebook failed")
	}

	// 粗略检查source ids是否存在且属于notebookid
	if len(params.SourceIds) > 0 {
		query := &bizsource.CheckSourceIdsQuery{
			NotebookId: params.NotebookId,
			SourceIds:  params.SourceIds,
		}
		existSourceIds, err := l.sourceBiz.CheckSourceIds(ctx, query)
		if err != nil {
			return emptyId, errors.WithMessage(err, "check source ids failed")
		}

		if len(existSourceIds) == 0 {
			return emptyId, errors.ErrParams.Msgf(
				"no source ids found, notebook_id=%s, source_ids=%v",
				params.NotebookId, params.SourceIds)
		}
	}

	chatId := params.NotebookId
	userMsgId, err := l.chatBiz.AddUserMessage(ctx, &bizchat.AddUserMessageCommand{
		ChatId:  chatId,
		UserId:  pkgcontext.GetUserId(ctx),
		Content: params.Prompt,
	})
	if err != nil {
		return emptyId, errors.WithMessage(err, "add user message failed")
	}

	einoMsg := &einoschema.Message{
		Role:    einoschema.User,
		Content: params.Prompt,
	}
	err = l.chatBiz.AppendContextMessage(ctx, chatId, []*einoschema.Message{einoMsg})
	if err != nil {
		return userMsgId, errors.WithMessage(err, "append context message failed")
	}

	// 启动后台处理流程
	ctx = context.WithoutCancel(ctx)
	l.wg.Add(1)
	go l.processUserMessageTask(ctx, params.NotebookId, userMsgId, params)

	return userMsgId, nil
}

type RetrieveSourceDocsParams struct {
	NotebookId uuid.UUID
	Prompt     string
	SourceIds  []uuid.UUID
}

func (l *ChatLogic) RetrieveSourceDocs(
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
			Count:      conf.Global().Logic.Chat.SourceDocsRecallCount,
		})
	if err != nil {
		return nil, errors.WithMessage(err, "recall source docs failed")
	}

	slog.DebugContext(ctx, fmt.Sprintf("successfully recalled %d source docs", len(retrieved)))

	return retrieved, nil
}

func (l *ChatLogic) processUserMessageTask(
	ctx context.Context,
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

	_, ok := l.processCheckSourceDocs(ctx, params.NotebookId, params.Prompt, params.SourceIds)
	if !ok {
		return
	}

	contextMsgs, ok := l.processCheckGetMessageContext(ctx, chatId)
	if !ok {
		return
	}

	chatAgent := l.buildNewChatAgent()
	answer, err := chatAgent.produceAnswer(ctx, chatId, contextMsgs)
	if err != nil {
		slog.ErrorContext(ctx, "chat agent loop failed",
			slog.String("chat_id", chatId.String()),
			slog.Any("err", err),
		)
	}

	if finalErr := l.finalizingProcess(ctx, chatId, answer, err); finalErr != nil {
		slog.ErrorContext(ctx, "finalizing process failed",
			slog.String("chat_id", chatId.String()),
			slog.Any("err", finalErr),
		)
	}
}

// 检查目标消息是否存在
func (l *ChatLogic) processCheckUserMessage(ctx context.Context, chatId, msgId uuid.UUID) bool {
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

	if targetMsg.MsgRole != model.ChatMessageRoleUser {
		slog.WarnContext(ctx, "target message is not a user message", logAttrs(nil)...)
		return false
	}

	return true
}

func (l *ChatLogic) processCheckSourceDocs(
	ctx context.Context,
	notebookId uuid.UUID,
	prompt string,
	sourceIds []uuid.UUID,
) ([]*model.SourceDoc, bool) {
	if len(sourceIds) == 0 {
		return nil, true
	}

	sourceDocs, err := l.RetrieveSourceDocs(ctx, &RetrieveSourceDocsParams{
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

func (l *ChatLogic) processCheckGetMessageContext(
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

func (l *ChatLogic) finalizingProcess(
	ctx context.Context,
	chatId uuid.UUID,
	answer *einoschema.Message,
	finalErr error,
) error {
	userId := pkgcontext.GetUserId(ctx)

	if finalErr != nil {
		_, err := l.chatBiz.AddAssistantSystemMessage(ctx, &bizchat.AddAssistantSystemMessageCommand{
			ChatId:  chatId,
			UserId:  userId,
			Content: "生成失败，请重试",
		})
		if err != nil {
			slog.ErrorContext(ctx, "add assistant system message failed",
				slog.String("chat_id", chatId.String()),
				slog.Any("err", err),
			)
		}
	}

	// 最终的结果落库
	_, err := l.chatBiz.AddAssistantMessage(ctx, &bizchat.AddAssistantMessageCommand{
		ChatId:           chatId,
		UserId:           userId,
		Content:          answer.Content,
		ReasoningContent: answer.ReasoningContent,
	})
	if err != nil {
		slog.ErrorContext(ctx,
			"add assistant message failed",
			slog.String("chat_id", chatId.String()),
			slog.Any("err", err),
		)
	}

	// TODO task 设置状态为finish + 延迟删除task
	// 
	// TODO stream event 写入final chunk + 延迟删除

	return nil
}

func (l *ChatLogic) buildNewChatAgent() *chatAgent {
	agent := newChatAgent(chatAgentConfig{
		maxRound:   15,
		llm:        l.llm,
		beforeChat: l.agentBeforeChatHook,
		msgAppender: func(ctx context.Context, chatId uuid.UUID, newMsgs []*einoschema.Message) {
			if err := l.chatBiz.AppendContextMessage(ctx, chatId, newMsgs); err != nil {
				slog.ErrorContext(ctx, "append context message failed",
					slog.String("chat_id", chatId.String()),
					slog.Any("err", err),
				)
			}
		},
	})

	// TODO bind tools

	return agent
}

func (l *ChatLogic) agentBeforeChatHook(
	ctx context.Context,
	chatId uuid.UUID,
	msgs []*einoschema.Message) (
	[]*einoschema.Message, error,
) {
	systemPrompt := &einoschema.Message{
		Role:    einoschema.System,
		Content: "You are a helpful assistant.",
	}

	newMsgs := make([]*einoschema.Message, len(msgs)+1)
	newMsgs[0] = systemPrompt
	copy(newMsgs[1:], msgs)

	return newMsgs, nil
}
