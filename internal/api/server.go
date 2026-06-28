package api

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic"
	chatlogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/chat"
	notebooklogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/notebook"
	sourcelogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/source"
	studiologic "github.com/gonotelm-lab/gonotelm/internal/app/logic/studio"
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

	notebookLogic *notebooklogic.Logic
	sourceLogic   *sourcelogic.Logic
	chatLogic     *chatlogic.Logic
	studioLogic   *studiologic.Logic

	getNotebookHandler        *notebookapp.GetNotebookHandler
	createNotebookHandler     *notebookapp.CreateNotebookHandler
	listNotebooksHandler      *notebookapp.ListNotebooksHandler
	deleteNotebookHandler     *notebookapp.DeleteNotebookHandler
	updateNotebookNameHandler *notebookapp.UpdateNotebookNameHandler

	// source handler
	getSourceHandler         *sourceapp.GetSourceHandler
	createSourceHandler      *sourceapp.CreateSourceHandler
	deleteSourceHandler      *sourceapp.DeleteSourceHandler
	presignUploadFileHandler *sourceapp.PresignUploadFileHandler
	pollSourceStatusHandler          *sourceapp.PollSourceStatusHandler
	retrySourcePreparationHandler    *sourceapp.RetrySourcePreparationHandler
	updateSourceTitleHandler         *sourceapp.UpdateSourceTitleHandler

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
		h:             hz,
		notebookLogic: logic.NotebookLogic,
		sourceLogic:   logic.SourceLogic,
		chatLogic:     logic.ChatLogic,
		studioLogic:   logic.StudioLogic,

		wire: wire,

		getNotebookHandler:        notebookapp.NewGetNotebookHandler(wire.NotebookRepo),
		createNotebookHandler:     notebookapp.NewCreateNotebookHandler(wire.NotebookRepo, wire.EventBus),
		listNotebooksHandler:      notebookapp.NewListNotebooksHandler(wire.NotebookRepo),
		deleteNotebookHandler:     notebookapp.NewDeleteNotebookHandler(wire.NotebookRepo, wire.EventBus),
		updateNotebookNameHandler: notebookapp.NewUpdateNotebookNameHandler(wire.NotebookRepo),

		getSourceHandler:         sourceapp.NewGetSourceHandler(wire.SourceRepo, wire.SourceStorageRepo),
		createSourceHandler:      sourceapp.NewCreateSourceHandler(wire.SourceRepo, wire.NotebookRepo, wire.EventBus),
		deleteSourceHandler:      sourceapp.NewDeleteSourceHandler(wire.SourceRepo, wire.EventBus),
		presignUploadFileHandler: sourceapp.NewPresignUploadFileHandler(wire.SourceRepo, wire.SourceStorageRepo),
		pollSourceStatusHandler:       sourceapp.NewPollSourceStatusHandler(wire.SourceRepo, wire.SourceStorageRepo, wire.EventBus),
		retrySourcePreparationHandler: sourceapp.NewRetrySourcePreparationHandler(wire.SourceRepo, wire.EventBus),
		updateSourceTitleHandler:      sourceapp.NewUpdateSourceTitleHandler(wire.SourceRepo),
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
