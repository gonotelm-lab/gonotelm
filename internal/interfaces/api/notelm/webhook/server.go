// Package webhook 提供外部系统回调 notelm 的内部端点，挂在根路由、不走 /api/v1 的 CSRF。
package webhook

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
)

type Server struct {
	cleaner *cleanup.Cleaner
}

func NewServer(cleaner *cleanup.Cleaner) *Server {
	return &Server{cleaner: cleaner}
}

func (s *Server) RegisterRoutes(h *server.Hertz) {
	s.registerFlowRoutes(h)
}
