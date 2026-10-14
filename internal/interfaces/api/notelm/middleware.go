package notelm

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/http/middleware"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
)

// csrfMiddleware 双提交校验；鉴权中间件在 auth 包里（auth.Server.Authenticate）
func (s *Server) csrfMiddleware() app.HandlerFunc {
	return middleware.CSRF(middleware.CSRFConfig{
		CookieName: schema.CSRFCookieName,
		HeaderName: schema.CSRFHeaderName,
		Secure:     true,
		SameSite:   protocol.CookieSameSiteLaxMode,
		OnError: func(_ context.Context, c *app.RequestContext) {
			http.ErrResp(c, pkgerrors.ErrCSRF)
		},
	})
}
