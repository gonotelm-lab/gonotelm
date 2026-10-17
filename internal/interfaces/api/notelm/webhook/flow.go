package webhook

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	httpadapter "github.com/gonotelm-lab/gonotelm/pkg/http/adapter"
)

// registerFlowRoutes 挂 flow 的清理回调：路径本身表明语义，载荷里的 kind 必须与之一致。
func (s *Server) registerFlowRoutes(h *server.Hertz) {
	h.POST(cleanup.SourceCallbackPath, s.flowCleanupHandler(cleanup.KindSource))
	h.POST(cleanup.NotebookCallbackPath, s.flowCleanupHandler(cleanup.KindNotebook))
	h.POST(cleanup.ArtifactCallbackPath, s.flowCleanupHandler(cleanup.KindArtifact))
}

// flowCleanupHandler 的 2xx 表示投递成功（含无需处理），5xx 让 flow 重投；清扫本身可重复执行。
func (s *Server) flowCleanupHandler(expectedKind string) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		req, err := httpadapter.GetCompatRequest(&c.Request, c.RemoteAddr().String())
		if err != nil {
			slog.WarnContext(ctx, "convert flow callback request failed", slog.Any("err", err))
			c.String(http.StatusBadRequest, "bad request")

			return
		}

		if err := s.flowClient.Verify(req); err != nil {
			slog.WarnContext(ctx, "flow callback verify failed", slog.Any("err", err))
			c.String(http.StatusUnauthorized, "unauthorized")

			return
		}

		body, err := io.ReadAll(req.Body) // verify会读body但是会还原 这里照常读入即可
		if err != nil {
			slog.WarnContext(ctx, "read flow callback body failed", slog.Any("err", err))
			c.String(http.StatusBadRequest, "read body failed")
			return
		}

		payload, err := cleanup.Unmarshal(body)
		if err != nil {
			// 可预期错误：重试也不会变好
			slog.WarnContext(ctx, "flow callback payload invalid", slog.Any("err", err))
			c.String(http.StatusOK, "ignored")

			return
		}
		if payload.Kind != expectedKind {
			slog.WarnContext(ctx, "flow callback kind mismatches route",
				slog.String("kind", payload.Kind),
				slog.String("expected", expectedKind),
			)
			c.String(http.StatusOK, "ignored")

			return
		}

		if err := s.sweeper.Sweep(ctx, payload); err != nil {
			// 临时失败：让 flow 重投，清扫本身可重复执行
			slog.ErrorContext(ctx, "flow callback sweep failed",
				slog.String("kind", payload.Kind),
				slog.String("notebook_id", payload.NotebookId.String()),
				slog.Any("err", err),
			)
			c.String(http.StatusServiceUnavailable, "retry later")

			return
		}

		c.String(http.StatusOK, "ok")
	}
}
