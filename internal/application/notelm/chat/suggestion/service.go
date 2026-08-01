package suggestion

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/shared"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
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

type Service struct {
	rootCtx            context.Context
	suggestionRepo     chatrepo.SuggestionRepository
	messageRepo        chatrepo.MessageRepository
	messageContextRepo chatrepo.ContextMessageRepository
	notebookRepo       notebookrepo.Repository
	sourceRepo         sourcerepo.Repository
	chatGateway        *llmchat.Gateway
}

func NewService(
	rootCtx context.Context,
	suggestionRepo chatrepo.SuggestionRepository,
	messageRepo chatrepo.MessageRepository,
	messageContextRepo chatrepo.ContextMessageRepository,
	notebookRepo notebookrepo.Repository,
	sourceRepo sourcerepo.Repository,
	chatGateway *llmchat.Gateway,
) *Service {
	return &Service{
		rootCtx:            rootCtx,
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

func (h *Service) Generate(ctx context.Context, cmd *GenerateSuggestionsCommand) (
	*GenerateSuggestionsResult, error,
) {
	var (
		chatId     valobj.Id   = cmd.Chat.Id
		notebookId valobj.Id   = cmd.Chat.NotebookId
		sourceIds  []valobj.Id = cmd.SourceIds
		userId     string      = cmd.UserId
	)

	// 处理逻辑如大致为：
	// 优先从contextMessage中获取上下文生成建议
	// 如果没有上下文 则查看最近的对话记录生成建议
	// 如果没有对话记录 则生成开场建议 开场建议按照来源和笔记本的信息生成建议
	recentContextMessages, err := h.messageContextRepo.ListRecent(ctx, chatId, maxMessageForSuggestionLimit)
	if err != nil {
		slog.ErrorContext(ctx, "suggestion failed to get recent context messages",
			slog.Any("err", err), slog.String("chat_id", chatId.String()),
		)
	}

	var chatMessages []*entity.Message
	if len(recentContextMessages) <= 0 {
		// 获取最近的对话记录
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

	if len(chatMessages) <= 0 {
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

const suggestPromptTemplate = `
The user has had several rounds of conversation with you in a notebook and now wants to continue asking questions. 
Based on the conversation history and the current context, generate 3 possible follow-up question suggestions. 
Relevant information is below:

Notebook name:
%s

---

Notebook sources: 
%s

---

Output question must be an array of strings in **JSON** format, for example:

["What is the capital of France?", "What is the capital of Germany?", "What is the capital of Italy?"]

---

The following is the conversation history between the user and you:

`

const suggestPromptTemplateWithoutHistory = `
Based on the notebook and sources information, generate 3 possible opener question suggestions. 

Notebook name:
%s

---

Notebook sources: 
%s

---

Output question must be an array of strings in **JSON** format, for example:

["What is the capital of France?", "What is the capital of Germany?", "What is the capital of Italy?"]

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
	var result []string
	s = strings.TrimSpace(s)
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
	msgs := make([]*einoschema.Message, 0, len(contextMessages)+1)
	msgs = append(msgs, nil)
	for _, cm := range contextMessages {
		msgs = append(msgs, cm.Message)
	}

	notebookDesc, sourceDesc := renderNotebookAndSources(notebook, sources)
	systemMsg := einoschema.SystemMessage(fmt.Sprintf(suggestPromptTemplate, notebookDesc, sourceDesc))
	msgs[0] = systemMsg

	return msgs
}

func (h *Service) renderFollowUpsPromptFromChatMessages(
	notebook *notebookentity.Notebook,
	sources []*sourceentity.Source,
	chatMessages []*entity.Message,
) []*einoschema.Message {
	msgs := make([]*einoschema.Message, 0, len(chatMessages)+1)
	msgs = append(msgs, nil)
	for _, cm := range chatMessages {
		msgs = append(msgs, cm.AsEinoMessage())
	}

	notebookDesc, sourceDesc := renderNotebookAndSources(notebook, sources)
	systemMsg := einoschema.SystemMessage(fmt.Sprintf(suggestPromptTemplate, notebookDesc, sourceDesc))
	msgs[0] = systemMsg

	return msgs
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
