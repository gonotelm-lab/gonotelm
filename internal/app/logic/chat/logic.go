package chat

import (
	"context"
	"log/slog"
	"math"
	"sync"

	bizchat "github.com/gonotelm-lab/gonotelm/internal/app/biz/chat"
	biznotebook "github.com/gonotelm-lab/gonotelm/internal/app/biz/notebook"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	chatmodel "github.com/gonotelm-lab/gonotelm/internal/app/model/chat"
	"github.com/gonotelm-lab/gonotelm/internal/app/prompt"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/rerank"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	einoschema "github.com/cloudwego/eino/schema"
)

const defaultPromptLang = "zh"

type Logic struct {
	wg                 sync.WaitGroup
	notebookBiz        *biznotebook.Biz
	sourceBiz          *bizsource.Biz
	chatBiz            *bizchat.Biz
	eventManager       *bizchat.ChatEventManager
	sourceDocRetriever *SourceDocRetriever

	llmGateway          *gateway.Gateway
	chatTemplateManager *prompt.ChatTemplateManager
}

func MustNewLogic(
	llmGateway *gateway.Gateway,
	rerankerGateway *rerank.Gateway,
	notebookBiz *biznotebook.Biz,
	sourceBiz *bizsource.Biz,
	agentSourceBiz *bizsource.AgentBiz,
	chatBiz *bizchat.Biz,
	eventManager *bizchat.ChatEventManager,
) *Logic {
	chatTemplateManager, err := prompt.NewChatTemplateManager(defaultPromptLang)
	if err != nil {
		panic(err)
	}

	logic := &Logic{
		notebookBiz:         notebookBiz,
		sourceBiz:           sourceBiz,
		chatBiz:             chatBiz,
		eventManager:        eventManager,
		llmGateway:          llmGateway,
		chatTemplateManager: chatTemplateManager,
		sourceDocRetriever:  NewSourceDocRetriever(sourceBiz, agentSourceBiz, llmGateway, rerankerGateway),
	}

	return logic
}

func (l *Logic) Close(ctx context.Context) {
	l.wg.Wait()
	slog.InfoContext(ctx, "chat logic closed")
}

type CreateUserMessageParams struct {
	ChatId            uuid.UUID
	Prompt            string
	SourceIds         []uuid.UUID
	EnableThinking    bool
	ChatStyle         chatmodel.ChatStyle
	ChatAnswerLength  chatmodel.ChatAnswerLength
	EnhancedRetrieval bool
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
		return nil, errors.ErrParams.Msgf("chat not belong to user, chat_id=%s, user_id=%s",
			params.ChatId, userId)
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
		query := &bizsource.CheckSourceIdsReadyQuery{
			NotebookId: targetChat.NotebookId,
			SourceIds:  params.SourceIds,
		}
		existSourceIds, err := l.sourceBiz.CheckSourceIdsReady(ctx, query)
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
	l.wg.Go(func() {
		l.processUserMessageTask(ctx, targetChat, streamTask.Id, userMsgId, params)
	})

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
