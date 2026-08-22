package notelm

import (
	"context"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/ulid"

	"github.com/cloudwego/hertz/pkg/app"
)

const devFixedUserId = "01hf7yat00vtpvxvyaztxbw001"

func (s *Server) authMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// TODO for test here we injust test user

		ctx = pkgcontext.WithUserId(ctx, ulid.MustParseString(devFixedUserId))

		c.Next(ctx)
	}
}
