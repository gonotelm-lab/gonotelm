package api

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic"
	studiologic "github.com/gonotelm-lab/gonotelm/internal/app/logic/studio"
	chatapp "github.com/gonotelm-lab/gonotelm/internal/application/chat"
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notebook"
	sourceapp "github.com/gonotelm-lab/gonotelm/internal/application/source"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infra"
	wire "github.com/gonotelm-lab/gonotelm/internal/wire"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/http/middleware"
)

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

	wire *wire.Wire
}

func NewServer(
	logic *logic.Logic,
	infras *infra.Instances,
	wire *wire.Wire,
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

		wire: wire,

		checkNotebookAccessHandler: notebookapp.NewCheckNotebookAccessHandler(wire.NotebookRepo),
		getNotebookHandler:         notebookapp.NewGetNotebookHandler(wire.NotebookRepo),
		createNotebookHandler:      notebookapp.NewCreateNotebookHandler(wire.NotebookRepo, wire.EventBus),
		listNotebooksHandler:       notebookapp.NewListNotebooksHandler(wire.NotebookRepo),
		deleteNotebookHandler:      notebookapp.NewDeleteNotebookHandler(wire.NotebookRepo, wire.EventBus),
		updateNotebookNameHandler:  notebookapp.NewUpdateNotebookNameHandler(wire.NotebookRepo),

		checkSourceAccessHandler:      sourceapp.NewCheckSourceAccessHandler(wire.SourceRepo),
		getSourceHandler:              sourceapp.NewGetSourceHandler(wire.SourceRepo, wire.SourceStorageRepo),
		createSourceHandler:           sourceapp.NewCreateSourceHandler(wire.SourceRepo, wire.NotebookRepo, wire.EventBus),
		deleteSourceHandler:           sourceapp.NewDeleteSourceHandler(wire.SourceRepo, wire.EventBus),
		presignUploadFileHandler:      sourceapp.NewPresignUploadFileHandler(wire.SourceRepo, wire.SourceStorageRepo),
		pollSourceStatusHandler:       sourceapp.NewPollSourceStatusHandler(wire.SourceRepo, wire.SourceStorageRepo, wire.EventBus),
		retrySourcePreparationHandler: sourceapp.NewRetrySourcePreparationHandler(wire.SourceRepo, wire.EventBus),
		updateSourceTitleHandler:      sourceapp.NewUpdateSourceTitleHandler(wire.SourceRepo),

		getSourceDocHandler:      sourceapp.NewGetSourceDocHandler(wire.SourceRepo, wire.SourceDocRepo),
		batchGetSourceDocHandler: sourceapp.NewBatchGetSourceDocsHandler(wire.SourceRepo, wire.SourceDocRepo),

		createChatHandler:  chatapp.NewCreateChatHandler(wire.NotebookRepo, wire.ChatRepo),
		listSourcesHandler: sourceapp.NewListSourcesHandler(wire.NotebookRepo, wire.SourceRepo, wire.SourceStorageRepo),

		chatCreateMessageHandler: chatapp.NewCreateMessageHandler(
			wire.WaitGroup,
			wire.NotebookRepo,
			wire.ChatRepo,
			wire.MessageRepo,
			wire.ContextMessageRepo,
			wire.StreamTaskRepo,
			wire.SourceRepo,
			wire.SourceStorageRepo,
			wire.SourceDocRepo,
			wire.Gateway(),
		),
		listMessagesHandler: chatapp.NewListMessagesHandler(
			wire.ChatRepo,
			wire.MessageRepo,
		),
		getStreamHandler:   chatapp.NewGetStreamHandler(wire.StreamTaskRepo),
		abortStreamHandler: chatapp.NewAbortStreamHandler(wire.StreamTaskRepo),
		deleteChatContextHandler: chatapp.NewDeleteChatContextHandler(
			wire.ChatRepo,
			wire.ContextMessageRepo,
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
