// Package webhook 提供外部系统回调 notelm 的内部端点，挂在根路由、不走 /api/v1 的 CSRF。
package webhook

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/gonotelm-lab/gonotelm/internal/application/notelm/source/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
)

type Server struct {
	flowClient flow.ScheduleClient
	sweeper    *cleanup.Sweeper
}

func NewServer(flowClient flow.ScheduleClient, sweeper *cleanup.Sweeper) *Server {
	return &Server{flowClient: flowClient, sweeper: sweeper}
}

func (s *Server) RegisterRoutes(h *server.Hertz) {
	s.registerFlowRoutes(h)
}
