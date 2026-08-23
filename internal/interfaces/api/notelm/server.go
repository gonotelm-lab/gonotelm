package notelm

import (
	stdcontext "context"
	"sync"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	artifactapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/artifact"
	chatapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat"
	chatsuggest "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/suggestion"
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/notebook"
	sourceapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/source"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/http/middleware"
)

type ServerDeps struct {
	RootCtx stdcontext.Context

	NotebookRepo           notebookrepo.Repository
	SourceRepo             sourcerepo.Repository
	SourceStorageRepo      sourcerepo.StorageRepository
	SourceDocRepo          sourcerepo.SourceDocRepository
	ChatRepo               chatrepo.ChatRepository
	ChatMessageRepo        chatrepo.MessageRepository
	ChatContextMessageRepo chatrepo.ContextMessageRepository
	ChatStreamTaskRepo     chatrepo.StreamTaskRepository
	ChatSuggestionRepo     chatrepo.SuggestionRepository
	ChatSuggestService     *chatsuggest.Service
	EventBus               eventbus.EventBus
	WaitGroup              *sync.WaitGroup
	LLMGateway             *chat.Gateway

	ArtifactRepo   artifactrepo.Repository
	FlowClient     flow.TaskClient
	Poller         artifactapp.Poller
	StorageGateway adapter.StorageAdapter

	TitleMaker adapter.TitleMaker
}

type Server struct {
	h *server.Hertz

	getNotebookHandler        *notebookapp.GetNotebookHandler
	createNotebookHandler     *notebookapp.CreateNotebookHandler
	listNotebooksHandler      *notebookapp.ListNotebooksHandler
	deleteNotebookHandler     *notebookapp.DeleteNotebookHandler
	updateNotebookNameHandler *notebookapp.UpdateNotebookNameHandler

	getSourceHandler              *sourceapp.GetSourceHandler
	createSourceHandler           *sourceapp.CreateSourceHandler
	deleteSourceHandler           *sourceapp.DeleteSourceHandler
	presignUploadFileHandler      *sourceapp.PresignUploadFileHandler
	pollSourceStatusHandler       *sourceapp.PollSourceStatusHandler
	retrySourcePreparationHandler *sourceapp.RetrySourcePreparationHandler
	updateSourceTitleHandler      *sourceapp.UpdateSourceTitleHandler

	getSourceDocHandler      *sourceapp.GetSourceDocHandler
	batchGetSourceDocHandler *sourceapp.BatchGetSourceDocsHandler

	listSourcesHandler *sourceapp.ListSourcesHandler
	createChatHandler  *chatapp.CreateChatHandler

	createChatMessageHandler  *chatapp.CreateMessageHandler
	listChatMessagesHandler   *chatapp.ListMessagesHandler
	getChatStreamHandler      *chatapp.GetStreamHandler
	abortChatStreamHandler    *chatapp.AbortStreamHandler
	deleteChatContextHandler  *chatapp.DeleteChatContextHandler
	getChatSuggestionsHandler *chatapp.ChatSuggestHandler

	generateArtifactHandler      *artifactapp.GenerateArtifactHandler
	getArtifactStatusHandler     *artifactapp.GetArtifactStatusHandler
	listNotebookArtifactsHandler *artifactapp.ListArtifactsHandler
	cancelArtifactHandler        *artifactapp.CancelArtifactHandler
	deleteArtifactHandler        *artifactapp.DeleteArtifactHandler
	retryArtifactHandler         *artifactapp.RetryArtifactHandler
	updateArtifactHandler        *artifactapp.UpdateArtifactHandler
	convertNoteToSourceHandler   *artifactapp.ConvertNoteToSourceHandler
}

func NewServer(
	deps ServerDeps,
) *Server {
	hzOpts := []config.Option{
		server.WithCustomBinder(http.NewCanonicalBinder()),
		server.WithHostPorts(conf.NotelmGlobal().Api.HostPort()),
		server.WithExitWaitTime(conf.NotelmGlobal().Api.ExitWaitTimeout),
		server.WithDisablePrintRoute(true),
	}
	hz := server.New(hzOpts...)
	hz.Use(
		middleware.Tracing("notelm"),
		middleware.Recovery(),
		middleware.Logging(middleware.WithLogAllError(conf.NotelmGlobal().IsDev())),
	)

	s := &Server{
		h:                         hz,
		getNotebookHandler:        notebookapp.NewGetNotebookHandler(deps.NotebookRepo),
		createNotebookHandler:     notebookapp.NewCreateNotebookHandler(deps.NotebookRepo, deps.EventBus),
		listNotebooksHandler:      notebookapp.NewListNotebooksHandler(deps.NotebookRepo),
		deleteNotebookHandler:     notebookapp.NewDeleteNotebookHandler(deps.NotebookRepo, deps.EventBus),
		updateNotebookNameHandler: notebookapp.NewUpdateNotebookNameHandler(deps.NotebookRepo),

		getSourceHandler:              sourceapp.NewGetSourceHandler(deps.SourceRepo, deps.SourceStorageRepo),
		createSourceHandler:           sourceapp.NewCreateSourceHandler(deps.SourceRepo, deps.NotebookRepo, deps.EventBus),
		deleteSourceHandler:           sourceapp.NewDeleteSourceHandler(deps.SourceRepo, deps.EventBus),
		presignUploadFileHandler:      sourceapp.NewPresignUploadFileHandler(deps.SourceRepo, deps.SourceStorageRepo),
		pollSourceStatusHandler:       sourceapp.NewPollSourceStatusHandler(deps.SourceRepo, deps.SourceStorageRepo, deps.EventBus),
		retrySourcePreparationHandler: sourceapp.NewRetrySourcePreparationHandler(deps.SourceRepo, deps.EventBus),
		updateSourceTitleHandler:      sourceapp.NewUpdateSourceTitleHandler(deps.SourceRepo),

		getSourceDocHandler:      sourceapp.NewGetSourceDocHandler(deps.SourceRepo, deps.SourceDocRepo),
		batchGetSourceDocHandler: sourceapp.NewBatchGetSourceDocsHandler(deps.SourceRepo, deps.SourceDocRepo),

		createChatHandler:  chatapp.NewCreateChatHandler(deps.NotebookRepo, deps.ChatRepo),
		listSourcesHandler: sourceapp.NewListSourcesHandler(deps.NotebookRepo, deps.SourceRepo, deps.SourceStorageRepo),
		getChatSuggestionsHandler: chatapp.NewChatSuggestHandler(
			deps.ChatRepo,
			deps.ChatSuggestService,
		),

		createChatMessageHandler: chatapp.NewCreateMessageHandler(
			deps.WaitGroup,
			deps.NotebookRepo,
			deps.ChatRepo,
			deps.ChatMessageRepo,
			deps.ChatContextMessageRepo,
			deps.ChatStreamTaskRepo,
			deps.SourceRepo,
			deps.SourceStorageRepo,
			deps.SourceDocRepo,
			deps.LLMGateway,
			deps.EventBus,
		),
		listChatMessagesHandler: chatapp.NewListMessagesHandler(
			deps.ChatRepo,
			deps.ChatMessageRepo,
		),
		getChatStreamHandler:   chatapp.NewGetStreamHandler(deps.ChatStreamTaskRepo),
		abortChatStreamHandler: chatapp.NewAbortStreamHandler(deps.ChatStreamTaskRepo, deps.EventBus),
		deleteChatContextHandler: chatapp.NewDeleteChatContextHandler(
			deps.ChatRepo,
			deps.ChatContextMessageRepo,
			deps.ChatSuggestionRepo,
		),

		generateArtifactHandler: artifactapp.NewGenerateArtifactHandler(
			deps.WaitGroup,
			deps.ArtifactRepo,
			deps.NotebookRepo,
			deps.SourceRepo,
			deps.ChatRepo,
			deps.ChatMessageRepo,
			deps.FlowClient,
			deps.Poller,
			deps.EventBus,
			deps.TitleMaker,
		),
		getArtifactStatusHandler:     artifactapp.NewGetArtifactStatusHandler(deps.ArtifactRepo, deps.FlowClient, deps.StorageGateway),
		listNotebookArtifactsHandler: artifactapp.NewListArtifactsHandler(deps.NotebookRepo, deps.ArtifactRepo),
		cancelArtifactHandler:        artifactapp.NewCancelArtifactHandler(deps.ArtifactRepo, deps.FlowClient, deps.EventBus),
		deleteArtifactHandler:        artifactapp.NewDeleteArtifactHandler(deps.ArtifactRepo, deps.FlowClient, deps.StorageGateway),
		retryArtifactHandler:         artifactapp.NewRetryArtifactHandler(deps.ArtifactRepo, deps.FlowClient, deps.Poller, deps.EventBus),
		updateArtifactHandler:        artifactapp.NewUpdateArtifactHandler(deps.ArtifactRepo),
		convertNoteToSourceHandler: artifactapp.NewConvertNoteToSourceHandler(
			deps.ArtifactRepo,
			deps.SourceRepo,
			deps.NotebookRepo,
			deps.SourceStorageRepo,
			deps.EventBus,
		),
	}

	s.registerRoutes()

	return s
}

func (s *Server) registerRoutes() {
	v1Group := s.h.Group("/api/v1", s.authMiddleware())

	s.registerNotebooksRoutes(v1Group)
	s.registerSourcesRoutes(v1Group)
	s.registerChatRoutes(v1Group)
	s.registerStudioRoutes(v1Group)
}

func (s *Server) Hertz() *server.Hertz { return s.h }

func (s *Server) Run() {
	s.h.Spin()
}
