package api

import (
	"context"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"

	"github.com/cloudwego/hertz/pkg/app"
)

func (s *Server) authMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// TODO for test here we injust test user

		ctx = pkgcontext.WithUserId(ctx, "test_user")

		c.Next(ctx)
	}
}
