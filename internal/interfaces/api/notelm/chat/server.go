package chat

import (
	"sync"

	chatapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat"
	chatsuggest "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/suggestion"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
)

// Deps 会话接口所需依赖，由外层 notelm.ServerDeps 裁剪后传入
type Deps struct {
	WaitGroup              *sync.WaitGroup
	DistLock               adapter.DistributedLock
	NotebookRepo           notebookrepo.Repository
	NotebookAccessService  *notebookservice.AccessService
	ChatRepo               chatrepo.ChatRepository
	ChatMessageRepo        chatrepo.MessageRepository
	ChatContextMessageRepo chatrepo.ContextMessageRepository
	ChatStreamTaskRepo     chatrepo.StreamTaskRepository
	ChatSuggestionRepo     chatrepo.SuggestionRepository
	SourceRepo             sourcerepo.Repository
	ObjectStore            adapter.ObjectStore
	SourceDocRepo          sourcerepo.SourceDocRepository
	LLMGateway             *llmchat.Gateway
	EventBus               eventbus.Publisher
	ChatSuggestService     *chatsuggest.Service
}

// Server 会话接口的处理器集合
type Server struct {
	createChatHandler *chatapp.CreateChatHandler

	createChatMessageHandler  *chatapp.CreateMessageHandler
	listChatMessagesHandler   *chatapp.ListMessagesHandler
	getChatStreamHandler      *chatapp.GetStreamHandler
	getChatRunningTaskHandler *chatapp.GetRunningTaskHandler
	abortChatStreamHandler    *chatapp.AbortStreamHandler
	deleteChatContextHandler  *chatapp.DeleteChatContextHandler
	getChatSuggestionsHandler *chatapp.ChatSuggestHandler
}

func NewServer(deps Deps) *Server {
	return &Server{
		createChatHandler: chatapp.NewCreateChatHandler(deps.NotebookAccessService, deps.ChatRepo),

		createChatMessageHandler: chatapp.NewCreateMessageHandler(
			deps.WaitGroup,
			deps.DistLock,
			deps.NotebookRepo,
			deps.NotebookAccessService,
			deps.ChatRepo,
			deps.ChatMessageRepo,
			deps.ChatContextMessageRepo,
			deps.ChatStreamTaskRepo,
			deps.SourceRepo,
			deps.ObjectStore,
			deps.SourceDocRepo,
			deps.LLMGateway,
			deps.EventBus,
		),
		listChatMessagesHandler: chatapp.NewListMessagesHandler(
			deps.ChatRepo,
			deps.NotebookAccessService,
			deps.ChatMessageRepo,
		),
		getChatStreamHandler: chatapp.NewGetStreamHandler(deps.ChatRepo, deps.NotebookAccessService, deps.ChatStreamTaskRepo),
		getChatRunningTaskHandler: chatapp.NewGetRunningTaskHandler(
			deps.ChatRepo,
			deps.NotebookAccessService,
			deps.ChatStreamTaskRepo,
		),
		abortChatStreamHandler: chatapp.NewAbortStreamHandler(deps.ChatRepo, deps.NotebookAccessService, deps.ChatStreamTaskRepo, deps.EventBus),
		deleteChatContextHandler: chatapp.NewDeleteChatContextHandler(
			deps.ChatRepo,
			deps.NotebookAccessService,
			deps.ChatContextMessageRepo,
			deps.ChatSuggestionRepo,
		),
		getChatSuggestionsHandler: chatapp.NewChatSuggestHandler(
			deps.ChatRepo,
			deps.NotebookAccessService,
			deps.ChatSuggestService,
		),
	}
}
