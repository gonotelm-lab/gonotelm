package studio

import (
	"github.com/cloudwego/hertz/pkg/route"
)

// RegisterRoutes 产物自身的路由，另外把 /notebooks/:id/artifacts 也注册在这里：
// 该端点是笔记本详情页的产物列表与生成入口。
func (s *Server) RegisterRoutes(g *route.RouterGroup) {
	notebookIdGroup := g.Group("/notebooks/:id")
	{
		// GET /api/v1/notebooks/:id/artifacts
		notebookIdGroup.GET("/artifacts", s.ListNotebookStudioArtifacts)
		// POST /api/v1/notebooks/:id/artifacts
		notebookIdGroup.POST("/artifacts", s.GenerateStudioArtifact)
	}

	// GET /api/v1/artifacts/style-previews?kind=slides
	g.GET("/artifacts/style-previews", s.ListStylePreviews)

	artifactGroup := g.Group("/artifacts/:id")
	{
		// GET /api/v1/artifacts/:id
		artifactGroup.GET("", s.GetStudioArtifact)
		// GET /api/v1/artifacts/:id/status
		artifactGroup.GET("/status", s.GetStudioArtifactStatus)
		// DELETE /api/v1/artifacts/:id
		artifactGroup.DELETE("", s.DeleteStudioArtifact)
		// PATCH /api/v1/artifacts/:id
		artifactGroup.PATCH("", s.UpdateStudioArtifact)
		// POST /api/v1/artifacts/:id/retry
		artifactGroup.POST("/retry", s.RetryStudioArtifactTask)
		// POST /api/v1/artifacts/:id/cancel
		artifactGroup.POST("/cancel", s.CancelStudioArtifactTask)
		// POST /api/v1/artifacts/:id/convert
		artifactGroup.POST("/convert", s.ConvertNoteToSource)
	}
}
