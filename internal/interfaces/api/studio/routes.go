package studio

import "github.com/cloudwego/hertz/pkg/app/server"

func RegisterRoutes(h *server.Hertz, deps *Deps) {
	g := h.Group("/studio/artifact/:task_id", deps.CheckArtifactAccess)
	{
		g.GET("/status", deps.GetStatus)
		g.GET("/result", deps.GetResult)
		g.POST("/delete", deps.Delete)
		g.POST("/retry", deps.Retry)
		g.POST("/cancel", deps.Cancel)
	}
	h.POST("/studio/artifact/generate", deps.Generate)
	h.GET("/notebook/:id/studio/artifact/list", deps.ListNotebookArtifacts)
}
