package api

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic"
	chatlogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/chat"
	sourcelogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/source"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/http/middleware"
)

type Server struct {
	h *server.Hertz

	notebookLogic *logic.NotebookLogic
	sourceLogic   *sourcelogic.SourceLogic
	chatLogic     *chatlogic.Logic
}

func NewServer(logic *logic.Logic) *Server {
	hz := server.Default(
		server.WithCustomBinder(http.NewCanonicalBinder()),
		server.WithHostPorts(conf.Global().Api.HostPort()),
		server.WithExitWaitTime(conf.Global().Api.ExitWaitTimeout),
		server.WithDisablePrintRoute(true),
	)
	hz.Use(middleware.LogRequest(
		middleware.WithLogAllError(conf.Global().IsDev()),
	))

	s := &Server{
		h:             hz,
		notebookLogic: logic.NotebookLogic,
		sourceLogic:   logic.SourceLogic,
		chatLogic:     logic.ChatLogic,
	}

	s.registerRoutes()

	return s
}

func (s *Server) registerRoutes() {
	v1Group := s.h.Group("/api/v1", s.authMiddleware()) // TODO add auth group middleware

	s.registerNotebooksRoutes(v1Group)
	s.registerSourcesRoutes(v1Group)
	s.registerChatRoutes(v1Group)
	s.registerInsightsRoutes(v1Group)
}

func (s *Server) Run() {
	s.h.Spin()
}
