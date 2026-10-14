package chat

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/http/middleware"

	"github.com/cloudwego/hertz/pkg/route"
)

// RegisterRoutes 会话自身的路由，另外把 /notebooks/:id/chats 也注册在这里：
// 该端点返回的是笔记本的默认会话。
func (s *Server) RegisterRoutes(g *route.RouterGroup) {
	notebookIdGroup := g.Group("/notebooks/:id")
	{
		// POST /api/v1/notebooks/:id/chats
		notebookIdGroup.POST("/chats", s.GetOrCreateNotebookChat)
	}

	chatIdGroup := g.Group("/chats/:id")
	{
		// GET /api/v1/chats/:id/messages
		chatIdGroup.GET("/messages", s.ListChatMessages)
		// POST /api/v1/chats/:id/messages
		chatIdGroup.POST("/messages", s.ChatCreateMessage)
		// POST /api/v1/chats/:id/stream/abort
		chatIdGroup.POST("/stream/abort", s.ChatAbortStream)
		// GET /api/v1/chats/:id/stream-task — current running task id
		chatIdGroup.GET("/stream-task", s.GetChatRunningTask)
		// GET /api/v1/chats/:id/stream — SSE
		chatIdGroup.GET("/stream", middleware.SlowRequestThreshold(120*time.Second), s.GetChatStream)
		// DELETE /api/v1/chats/:id/context
		chatIdGroup.DELETE("/context", s.DeleteChatContext)
		// GET /api/v1/chats/:id/suggestions
		chatIdGroup.GET("/suggestions", s.GetChatSuggestions)
	}
}
