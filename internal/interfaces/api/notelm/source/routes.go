package source

import (
	"github.com/cloudwego/hertz/pkg/route"
)

// RegisterRoutes 来源自身的路由，另外把 /notebooks/:id/sources 也注册在这里：
// 该端点操作的是来源聚合。
func (s *Server) RegisterRoutes(g *route.RouterGroup) {
	notebookIdGroup := g.Group("/notebooks/:id")
	{
		// GET /api/v1/notebooks/:id/sources
		notebookIdGroup.GET("/sources", s.ListNotebookSources)
		// POST /api/v1/notebooks/:id/sources
		notebookIdGroup.POST("/sources", s.CreateSource)
	}

	sourceIdGroup := g.Group("/sources/:id")
	{
		// GET /api/v1/sources/:id
		sourceIdGroup.GET("", s.GetSource)
		// DELETE /api/v1/sources/:id
		sourceIdGroup.DELETE("", s.DeleteSource)
		// PATCH /api/v1/sources/:id
		sourceIdGroup.PATCH("", s.UpdateSource)
		// POST /api/v1/sources/:id/uploads
		sourceIdGroup.POST("/uploads", s.UploadFileSource)
		// POST /api/v1/sources/:id/poll — may advance uploading → preparing
		sourceIdGroup.POST("/poll", s.PollSourceStatus)
		// POST /api/v1/sources/:id/retry
		sourceIdGroup.POST("/retry", s.RetrySourcePreparation)
		// GET /api/v1/sources/:id/docs
		sourceIdGroup.GET("/docs", s.BatchGetSourceDocs)
		// GET /api/v1/sources/:id/docs/:doc_id
		sourceIdGroup.GET("/docs/:doc_id", s.GetSourceDoc)
	}
}
