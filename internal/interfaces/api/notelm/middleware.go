package notelm

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/http/middleware"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
)

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

func (s *Server) authMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		sid := string(c.Cookie(schema.AuthSessionCookieName))

		authn, err := s.authHandler.Authenticate(ctx, sid)
		if err != nil {
			if pkgerrors.Is(err, errors.ErrUserSessionNotFound) {
				http.ErrResp(c, pkgerrors.ErrNotLogin)
				return
			}

			http.ErrResp(c, err)
			return
		}

		ctx = pkgcontext.WithUserId(ctx, authn.UserId)
		c.Next(ctx)
	}
}
