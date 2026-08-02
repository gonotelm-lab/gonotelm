package suggestion

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/shared"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	domainerr "github.com/gonotelm-lab/gonotelm/internal/domain/chat/errors"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/safe"

	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
)

const maxMessageForSuggestionLimit = 100

const suggestionLockKeyPrefix = "gonotelm:suggestion:lock:"

func suggestionLockKey(chatId valobj.Id) string {
	return suggestionLockKeyPrefix + chatId.String()
}

type Service struct {
	rootCtx            context.Context
	distLock           adapter.DistributedLock
	suggestionRepo     chatrepo.SuggestionRepository
	messageRepo        chatrepo.MessageRepository
	messageContextRepo chatrepo.ContextMessageRepository
	notebookRepo       notebookrepo.Repository
	sourceRepo         sourcerepo.Repository
	chatGateway        *llmchat.Gateway
}

func NewService(
	rootCtx context.Context,
	distLock adapter.DistributedLock,
	suggestionRepo chatrepo.SuggestionRepository,
	messageRepo chatrepo.MessageRepository,
	messageContextRepo chatrepo.ContextMessageRepository,
	notebookRepo notebookrepo.Repository,
	sourceRepo sourcerepo.Repository,
	chatGateway *llmchat.Gateway,
) *Service {
	return &Service{
		rootCtx:            rootCtx,
		distLock:           distLock,
		suggestionRepo:     suggestionRepo,
		messageRepo:        messageRepo,
		messageContextRepo: messageContextRepo,
		notebookRepo:       notebookRepo,
		sourceRepo:         sourceRepo,
		chatGateway:        chatGateway,
	}
}

type GenerateSuggestionsCommand struct {
	Chat      *entity.Chat
	SourceIds []valobj.Id
	UserId    string
}

type GenerateSuggestionsResult struct {
	Questions      []string
	SuggestionType entity.SuggestionType
}

const (
	lockWaitCount    = 5
	lockWaitDuration = time.Second * 4
)

// Get 获取该 chat 已缓存的建议，未生成或不存在时返回空结果
func (h *Service) Get(ctx context.Context, chatId valobj.Id) (*GenerateSuggestionsResult, error) {
	// 先检查是否有锁 标识是否有suggestion正在生成 如果有的话 自旋等待一会
	for range lockWaitCount {
		hasLock, err := h.distLock.Check(ctx, suggestionLockKey(chatId))
		if err != nil {
			slog.ErrorContext(ctx, "suggestion get locked failed", slog.String("chat_id", chatId.String()))
			continue
		}
		
		if !hasLock {
			// 锁已释放，可以读取建议
			suggestions, err := h.suggestionRepo.Get(ctx, chatId)
			if err != nil {
				slog.ErrorContext(ctx, "suggestion failed to get suggestions",
					slog.Any("err", err), slog.String("chat_id", chatId.String()),
				)
				return &GenerateSuggestionsResult{}, nil
			}
			if suggestions == nil {
				return &GenerateSuggestionsResult{}, nil
			}

			return &GenerateSuggestionsResult{
				Questions:      suggestions.Questions,
				SuggestionType: suggestions.Type,
			}, nil
		}

		// 锁仍被占用，等待下一轮
		time.Sleep(lockWaitDuration)
	}

	// 自旋结束锁仍存在，说明建议正在生成中
	return nil, domainerr.ErrSuggestionGenerating
}

// Delete 删除该 chat 的缓存建议
func (h *Service) Delete(ctx context.Context, chatId valobj.Id) error {
	return h.suggestionRepo.Delete(ctx, chatId)
}

func (h *Service) Generate(ctx context.Context, cmd *GenerateSuggestionsCommand) (
	*GenerateSuggestionsResult, error,
) {
	var (
		chatId     = cmd.Chat.Id
		notebookId = cmd.Chat.NotebookId
		sourceIds  = cmd.SourceIds
		userId     = cmd.UserId
	)

	// 先锁住 可能此时客户端请求上来 可能导致同一段上下文被重复生成
	err := h.distLock.Lock(ctx, suggestionLockKey(cmd.Chat.Id))
	if err != nil {
		// 加锁失败直接放弃
		slog.ErrorContext(ctx, "suggestion failed to lock when generating",
			slog.Any("err", err), slog.String("chat_id", chatId.String()),
		)
		return nil, errors.WithMessagef(err, "suggestion generation lock failed")
	}
	defer func() {
		if err := h.distLock.Unlock(ctx, suggestionLockKey(cmd.Chat.Id)); err != nil {
			slog.ErrorContext(ctx, "suggestion failed to unlock when generating",
				slog.Any("err", err), slog.String("chat_id", chatId.String()),
			)
		}
	}()

	// 处理逻辑如大致为：
	// 优先从contextMessage中获取上下文生成建议
	// 如果没有上下文 则查看最近的对话记录生成建议
	// 如果没有对话记录 则生成开场建议 开场建议按照来源和笔记本的信息生成建议
	recentContextMessages, err := h.messageContextRepo.ListRecent(
		ctx,
		chatId,
		maxMessageForSuggestionLimit,
	)
	if err != nil {
		slog.ErrorContext(ctx, "suggestion failed to get recent context messages",
			slog.Any("err", err), slog.String("chat_id", chatId.String()),
		)
	}

	var chatMessages []*entity.Message
	if len(recentContextMessages) <= 0 {
		// 获取最近的对话记录（倒序：最新在前），渲染前翻转为时间正序
		chatMessages, err = h.messageRepo.ListByChatId(ctx, chatId, chatrepo.ListSpec{
			Offset: 0,
			Limit:  maxMessageForSuggestionLimit,
			Order:  chatrepo.ListSpecOrderSeqNoDesc,
		})
		if err != nil {
			slog.ErrorContext(ctx, "suggestion failed to get chat messages",
				slog.Any("err", err), slog.String("chat_id", chatId.String()),
			)
		}
		slices.Reverse(chatMessages)
	}

	targetNotebook, err := h.notebookRepo.FindById(ctx, notebookId)
	if err != nil {
		return nil, errors.WithMessagef(err, "suggestion failed to get notebook, notebook_id: %s", notebookId.String())
	}

	var targetSources []*sourceentity.Source
	if len(sourceIds) > 0 {
		targetSources, err = shared.FilterReadySources(ctx, h.sourceRepo, notebookId, sourceIds, userId)
		if err != nil {
			return nil, errors.WithMessagef(err, "suggestion failed to get sources, notebook_id: %s, source_ids: %v",
				notebookId.String(),
				sourceIds,
			)
		}
	}

	var (
		questions      []string
		suggestionType entity.SuggestionType
	)

	// 有历史对话则生成追问建议，否则生成开场建议
	hasHistory := len(recentContextMessages) > 0 || len(chatMessages) > 0
	if !hasHistory {
		// 生成开场建议
		suggestionType = entity.SuggestionTypeOpener
		questions, err = h.generateOpeners(ctx, targetNotebook, targetSources)
		if err != nil {
			return nil, errors.WithMessagef(err, "suggestion failed to generate opener, chat_id: %s", chatId.String())
		}
	} else {
		// 生成追问建议
		suggestionType = entity.SuggestionTypeFollowUp
		questions, err = h.generateFollowUps(ctx, targetNotebook, targetSources, recentContextMessages, chatMessages)
		if err != nil {
			return nil, errors.WithMessagef(err, "suggestion failed to generate follow up, chat_id: %s", chatId.String())
		}
	}

	h.saveSuggestions(ctx, chatId, suggestionType, questions)

	return &GenerateSuggestionsResult{
		Questions:      questions,
		SuggestionType: suggestionType,
	}, nil
}

const suggestPromptTemplate = `You are an assistant helping the user continue their conversation in a notebook. 
Based on the conversation history and the notebook/sources context below, generate exactly 3 short follow-up questions the user is likely to ask next.

The questions must:
- match the language of the conversation (if the conversation is in Chinese, output Chinese)
- be concise (ideally under 30 characters), diverse, and directly relevant to the conversation topic
- be phrased as natural user questions, as if the user asks them in the next turn

Notebook name:
%s

---

Notebook sources:
%s

---

Conversation history format: each message is prefixed with "## role" (user/assistant/tool), followed by its content.

---

Output requirements:
- Output ONLY a JSON array of strings, for example: ["What is the capital of France?", "What is the capital of Germany?", "What is the capital of Italy?"]
- No markdown code blocks, no explanations, no prefixes or suffixes.

`

const suggestPromptTemplateWithoutHistory = `You are an assistant helping the user start a conversation in a notebook. 
Based on the notebook and its sources below, generate exactly 3 opener questions that help the user begin exploring the material.

The questions must:
- match the language of the notebook/sources content
- be concise (ideally under 30 characters), diverse, and useful starting points (e.g. overall summary, key conclusions, specific topics covered by the sources)
- be phrased as natural user questions

Notebook name:
%s

---

Notebook sources:
%s

---

Output requirements:
- Output ONLY a JSON array of strings, for example: ["What is the capital of France?", "What is the capital of Germany?", "What is the capital of Italy?"]
- No markdown code blocks, no explanations, no prefixes or suffixes.

`

func (h *Service) getChatSuggestModel() (einomodel.ToolCallingChatModel, []einomodel.Option, error) {
	chatCfg := conf.NotelmGlobal().Chat
	providerType := chatCfg.ModelProvider
	opts := []einomodel.Option{
		einomodel.WithModel(chatCfg.Model),
		llmchat.WithThinking(providerType, false),
		llmchat.WithResponseJsonObject(providerType),
	}

	chatModel, err := h.chatGateway.GetProvider(providerType)
	if err != nil {
		return nil, nil, errors.Wrapf(errors.ErrInner, "model_provider=%s not found", providerType)
	}

	return chatModel, opts, nil
}

func parseChatModelOutput(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.ErrLLM.Msg("chat model returned empty output")
	}

	var result []string
	err := pkgjson.Unmarshal([]byte(s), &result)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "failed to unmarshal chat model output, err=%s", err.Error())
	}

	return result, nil
}

func (h *Service) callChatModelForResult(ctx context.Context, promptMsgs []*einoschema.Message) ([]string, error) {
	chatModel, opts, err := h.getChatSuggestModel()
	if err != nil {
		return nil, err
	}

	output, err := chatModel.Generate(ctx, promptMsgs, opts...)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrLLM, "failed to generate llm output, err=%s", err.Error())
	}

	result, err := parseChatModelOutput(output.Content)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrLLM, "failed to parse chat model output, err=%s", err.Error())
	}

	return result, nil
}

// 追问建议
func (h *Service) generateFollowUps(
	ctx context.Context,
	notebook *notebookentity.Notebook,
	sources []*sourceentity.Source,
	contextMessages []*entity.ContextMessage,
	chatMessages []*entity.Message,
) ([]string, error) {
	var promptMsgs []*einoschema.Message
	if len(contextMessages) > 0 {
		promptMsgs = h.renderFollowUpsPromptFromContextMessages(notebook, sources, contextMessages)
	} else {
		promptMsgs = h.renderFollowUpsPromptFromChatMessages(notebook, sources, chatMessages)
	}

	result, err := h.callChatModelForResult(ctx, promptMsgs)
	if err != nil {
		return nil, errors.WithMessage(err, "generate follow ups failed")
	}

	return result, nil
}

func renderNotebookAndSources(notebook *notebookentity.Notebook, sources []*sourceentity.Source) (string, string) {
	notebookDesc := notebook.NameAndDescription()
	var sourceDesc strings.Builder
	for idx, source := range sources {
		fmt.Fprintf(&sourceDesc, "%d. SourceTitle: %s\nSourceAbstract: %s\n", idx+1, source.Title, source.Abstract)
	}

	return notebookDesc, sourceDesc.String()
}

func (h *Service) renderFollowUpsPromptFromContextMessages(
	notebook *notebookentity.Notebook,
	sources []*sourceentity.Source,
	contextMessages []*entity.ContextMessage,
) []*einoschema.Message {
	history := make([]*einoschema.Message, 0, len(contextMessages))
	for _, cm := range contextMessages {
		if cm.Message != nil {
			history = append(history, cm.Message)
		}
	}
	historyText := renderConversationHistoryText(history)

	notebookDesc, sourceDesc := renderNotebookAndSources(notebook, sources)
	systemMsg := einoschema.SystemMessage(fmt.Sprintf(suggestPromptTemplate, notebookDesc, sourceDesc))
	userMsg := einoschema.UserMessage(historyText)

	return []*einoschema.Message{systemMsg, userMsg}
}

func (h *Service) renderFollowUpsPromptFromChatMessages(
	notebook *notebookentity.Notebook,
	sources []*sourceentity.Source,
	chatMessages []*entity.Message,
) []*einoschema.Message {
	history := make([]*einoschema.Message, 0, len(chatMessages))
	for _, cm := range chatMessages {
		if msg := cm.AsEinoMessage(); msg != nil {
			history = append(history, msg)
		}
	}
	historyText := renderConversationHistoryText(history)

	notebookDesc, sourceDesc := renderNotebookAndSources(notebook, sources)
	systemMsg := einoschema.SystemMessage(fmt.Sprintf(suggestPromptTemplate, notebookDesc, sourceDesc))
	userMsg := einoschema.UserMessage(historyText)

	return []*einoschema.Message{systemMsg, userMsg}
}

func renderConversationHistoryText(msgs []*einoschema.Message) string {
	var b strings.Builder
	for _, msg := range msgs {
		if msg == nil {
			continue
		}

		content := strings.TrimSpace(msg.Content)
		if content == "" && len(msg.ToolCalls) == 0 {
			continue
		}

		role := string(msg.Role)
		fmt.Fprintf(&b, "## %s\n%s\n\n", role, content)
	}
	return strings.TrimSpace(b.String())
}

// 开场建议
func (h *Service) generateOpeners(
	ctx context.Context,
	notebook *notebookentity.Notebook,
	sources []*sourceentity.Source,
) ([]string, error) {
	promptMsgs := make([]*einoschema.Message, 0, 1)
	notebookTips, sourcesTips := renderNotebookAndSources(notebook, sources)
	promptMsgs = append(promptMsgs,
		einoschema.SystemMessage(
			fmt.Sprintf(suggestPromptTemplateWithoutHistory, notebookTips, sourcesTips),
		),
	)

	result, err := h.callChatModelForResult(ctx, promptMsgs)
	if err != nil {
		return nil, errors.WithMessage(err, "generate opener failed")
	}

	return result, nil
}

func (h *Service) saveSuggestions(
	ctx context.Context,
	chatId valobj.Id,
	st entity.SuggestionType,
	questions []string,
) {
	safe.DetachGo(ctx, h.rootCtx, func(ctx context.Context) {
		if err := h.suggestionRepo.Save(ctx, chatId, entity.NewSuggestion(st, questions)); err != nil {
			slog.ErrorContext(ctx, "failed to save suggestions",
				slog.Any("err", err),
				slog.String("chat_id", chatId.String()),
			)
		}
	})
}
