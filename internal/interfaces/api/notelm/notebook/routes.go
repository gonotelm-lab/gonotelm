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
		// POST /api/v1/notebooks/:id/pin
		notebookIdGroup.POST("/pin", s.PinNotebook)
		// POST /api/v1/notebooks/:id/unpin
		notebookIdGroup.POST("/unpin", s.UnpinNotebook)
		// POST /api/v1/notebooks/:id/description/generation
		notebookIdGroup.POST("/description/generation", s.GenerateNotebookDescription)

		// GET /api/v1/notebooks/:id/access
		notebookIdGroup.GET("/access", s.GetNotebookAccess)

		// GET /api/v1/notebooks/:id/sharing
		notebookIdGroup.GET("/sharing", s.GetNotebookSharing)
		// PATCH /api/v1/notebooks/:id/visibility
		notebookIdGroup.PATCH("/visibility", s.UpdateNotebookVisibility)
		// PUT /api/v1/notebooks/:id/sharing/members
		notebookIdGroup.PUT("/sharing/members", s.UpsertNotebookMember)
		// DELETE /api/v1/notebooks/:id/sharing/members/:user_id
		notebookIdGroup.DELETE("/sharing/members/:user_id", s.DeleteNotebookMember)

		// POST /api/v1/notebooks/:id/cover/uploads
		notebookIdGroup.POST("/cover/uploads", s.CoverUploadPresign)
		// POST /api/v1/notebooks/:id/cover/confirm
		notebookIdGroup.POST("/cover/confirm", s.ConfirmCoverUpload)
		// DELETE /api/v1/notebooks/:id/cover
		notebookIdGroup.DELETE("/cover", s.DeleteNotebookCover)
	}
}
