package api

import (
	"sync"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic"
	studiologic "github.com/gonotelm-lab/gonotelm/internal/app/logic/studio"
	chatapp "github.com/gonotelm-lab/gonotelm/internal/application/chat"
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notebook"
	sourceapp "github.com/gonotelm-lab/gonotelm/internal/application/source"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/gateway"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/http/middleware"
)

type ServerDeps struct {
	NotebookRepo      notebookrepo.Repository
	SourceRepo        sourcerepo.Repository
	SourceStorageRepo sourcerepo.StorageRepository
	SourceDocRepo     sourcerepo.SourceDocRepository
	ChatRepo          chatrepo.Repository
	MessageRepo       chatrepo.MessageRepository
	ContextMessageRepo chatrepo.ContextMessageRepository
	StreamTaskRepo    chatrepo.StreamTaskRepository
	EventBus          eventbus.EventBus
	WaitGroup         *sync.WaitGroup
	Gateway           *gateway.Gateway
}

type Server struct {
	h *server.Hertz

	studioLogic *studiologic.Logic

	checkNotebookAccessHandler *notebookapp.CheckNotebookAccessHandler
	getNotebookHandler         *notebookapp.GetNotebookHandler
	createNotebookHandler      *notebookapp.CreateNotebookHandler
	listNotebooksHandler       *notebookapp.ListNotebooksHandler
	deleteNotebookHandler      *notebookapp.DeleteNotebookHandler
	updateNotebookNameHandler  *notebookapp.UpdateNotebookNameHandler

	// source handler
	checkSourceAccessHandler      *sourceapp.CheckSourceAccessHandler
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

	chatCreateMessageHandler *chatapp.CreateMessageHandler
	listMessagesHandler      *chatapp.ListMessagesHandler
	getStreamHandler         *chatapp.GetStreamHandler
	abortStreamHandler       *chatapp.AbortStreamHandler
	deleteChatContextHandler *chatapp.DeleteChatContextHandler
}

func NewServer(
	logic *logic.Logic,
	deps ServerDeps,
) *Server {
	hz := server.Default(
		server.WithCustomBinder(http.NewCanonicalBinder()),
		server.WithHostPorts(conf.Global().Api.HostPort()),
		server.WithExitWaitTime(conf.Global().Api.ExitWaitTimeout),
		server.WithDisablePrintRoute(true),
	)
	hz.Use(
		middleware.LogRequest(middleware.WithLogAllError(conf.Global().IsDev())),
	)

	s := &Server{
		h:           hz,
		studioLogic: logic.StudioLogic,

		checkNotebookAccessHandler: notebookapp.NewCheckNotebookAccessHandler(deps.NotebookRepo),
		getNotebookHandler:         notebookapp.NewGetNotebookHandler(deps.NotebookRepo),
		createNotebookHandler:      notebookapp.NewCreateNotebookHandler(deps.NotebookRepo, deps.EventBus),
		listNotebooksHandler:       notebookapp.NewListNotebooksHandler(deps.NotebookRepo),
		deleteNotebookHandler:      notebookapp.NewDeleteNotebookHandler(deps.NotebookRepo, deps.EventBus),
		updateNotebookNameHandler:  notebookapp.NewUpdateNotebookNameHandler(deps.NotebookRepo),

		checkSourceAccessHandler:      sourceapp.NewCheckSourceAccessHandler(deps.SourceRepo),
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

		chatCreateMessageHandler: chatapp.NewCreateMessageHandler(
			deps.WaitGroup,
			deps.NotebookRepo,
			deps.ChatRepo,
			deps.MessageRepo,
			deps.ContextMessageRepo,
			deps.StreamTaskRepo,
			deps.SourceRepo,
			deps.SourceStorageRepo,
			deps.SourceDocRepo,
			deps.Gateway,
		),
		listMessagesHandler: chatapp.NewListMessagesHandler(
			deps.ChatRepo,
			deps.MessageRepo,
		),
		getStreamHandler:   chatapp.NewGetStreamHandler(deps.StreamTaskRepo),
		abortStreamHandler: chatapp.NewAbortStreamHandler(deps.StreamTaskRepo),
		deleteChatContextHandler: chatapp.NewDeleteChatContextHandler(
			deps.ChatRepo,
			deps.ContextMessageRepo,
		),
	}

	s.registerRoutes()

	return s
}

func (s *Server) registerRoutes() {
	v1Group := s.h.Group("/api/v1", s.authMiddleware()) // TODO add auth group middleware

	s.registerNotebooksRoutes(v1Group)
	s.registerSourcesRoutes(v1Group)
	s.registerChatRoutes(v1Group)
	s.registerStudioRoutes(v1Group)
}

func (s *Server) Run() {
	s.h.Spin()
}
