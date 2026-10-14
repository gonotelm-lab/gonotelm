package notebook

import (
	"github.com/cloudwego/hertz/pkg/route"
)

// RegisterRoutes 笔记本自身的路由；/notebooks/:id 下的 sources、artifacts、chats
// 分别由 source、studio、chat 包注册到同一前缀下。
func (s *Server) RegisterRoutes(g *route.RouterGroup) {
	// POST /api/v1/notebooks
	g.POST("/notebooks", s.CreateNotebook)
	// GET /api/v1/notebooks
	g.GET("/notebooks", s.ListNotebooks)

	notebookIdGroup := g.Group("/notebooks/:id")
	{
		// GET /api/v1/notebooks/:id
		notebookIdGroup.GET("", s.GetNotebook)
		// DELETE /api/v1/notebooks/:id
		notebookIdGroup.DELETE("", s.DeleteNotebook)
		// PATCH /api/v1/notebooks/:id
		notebookIdGroup.PATCH("", s.UpdateNotebook)
		// POST /api/v1/notebooks/:id/description/generation
		notebookIdGroup.POST("/description/generation", s.GenerateNotebookDescription)
	}
}
