package api

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic"
	chatlogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/chat"
	notebooklogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/notebook"
	sourcelogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/source"
	studiologic "github.com/gonotelm-lab/gonotelm/internal/app/logic/studio"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/http/middleware"
)

type Server struct {
	h *server.Hertz

	notebookLogic *notebooklogic.Logic
	sourceLogic   *sourcelogic.Logic
	chatLogic     *chatlogic.Logic
	studioLogic   *studiologic.Logic
}

func NewServer(logic *logic.Logic) *Server {
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
