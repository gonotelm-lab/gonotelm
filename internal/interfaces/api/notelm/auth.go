package notelm

import (
	"context"
	stdhttp "net/http"

	authapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/auth"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route"
)

func (s *Server) registerAuthRoutes(g *route.RouterGroup) {
	authGroup := g.Group("/auth")
	{
		// POST /api/v1/auth/login
		authGroup.GET("/login", s.AuthLogin)
		// POST /api/v1/auth/logout
		authGroup.POST("/logout", s.AuthLogout)
		// GET /api/v1/auth/providers
		authGroup.GET("/providers", s.AuthProviders)

		// we use different callback endpoints for different providers
		callbackGroup := authGroup.Group("/callback")
		{
			// GET /api/v1/auth/callback/github
			callbackGroup.GET("/github", s.AuthCallback(entity.ProviderTypeGithub))
			// GET /api/v1/auth/callback/google
			callbackGroup.GET("/google", s.AuthCallback(entity.ProviderTypeGoogle))
		}
	}
}

func (s *Server) AuthLogin(ctx context.Context, c *app.RequestContext) {
	var req schema.AuthLoginRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	allowedOrigins := conf.NotelmGlobal().Auth.ReturnToAllowOrigins
	if !schema.IsSafeReturnTo(req.ReturnTo, allowedOrigins) {
		http.ErrResp(c, errors.ErrParams.Msgf("invalid return_to: %s", req.ReturnTo))
		return
	}

	var currentSessionId string
	if raw := c.Cookie(schema.AuthSessionCookieName); len(raw) > 0 {
		currentSessionId = string(raw)
	}

	resp, err := s.authLoginHandler.Handle(ctx, &authapp.LoginHandleCommand{
		LoginProvider:    entity.ProviderType(req.LoginProvider),
		LoginFrom:        entity.DeviceType(req.LoginFrom),
		ReturnTo:         req.ReturnTo,
		CurrentSessionId: currentSessionId,
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	// already logged in, skip oauth
	if resp.Authenticated {
		returnTo := resp.ReturnTo
		if returnTo == "" || !schema.IsSafeReturnTo(returnTo, allowedOrigins) {
			returnTo = "/"
		}
		c.Redirect(stdhttp.StatusFound, []byte(returnTo))
		return
	}

	c.SetCookie(schema.AuthLoginStateCookieName,
		resp.LoginInfo.State,
		schema.AuthLoginStateCookieTTL,
		"/",
		"",
		protocol.CookieSameSiteLaxMode,
		true,
		true,
	)

	// redirect to login url
	c.Redirect(stdhttp.StatusFound, []byte(resp.LoginInfo.URL))
}

func (s *Server) AuthLogout(ctx context.Context, c *app.RequestContext) {
	sid := string(c.Cookie(schema.AuthSessionCookieName))
	if err := s.authHandler.SignOut(ctx, sid); err != nil {
		http.ErrResp(c, err)
		return
	}

	c.SetCookie(schema.AuthSessionCookieName, "", -1, "/", "", protocol.CookieSameSiteLaxMode, true, true)
	c.SetCookie(schema.AuthUserIdCookieName, "", -1, "/", "", protocol.CookieSameSiteLaxMode, true, false)

	http.OkRespNoContent(c)
}

func (s *Server) AuthProviders(ctx context.Context, c *app.RequestContext) {
	names := s.authProvidersHandler.Handle()

	providers := make([]schema.AuthProvider, 0, len(names))
	for _, name := range names {
		providers = append(providers, schema.AuthProvider{Name: name})
	}

	http.OkResp(c, schema.AuthProvidersResponse{Providers: providers})
}

func (s *Server) AuthCallback(providerType entity.ProviderType) func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		var req schema.AuthCallbackRequest
		if err := c.BindAndValidate(&req); err != nil {
			http.ErrResp(c, err)
			return
		}

		// check if is error
		if req.IsError() {
			http.ErrResp(c, errors.ErrUnauthorized.Msgf("%s %s", req.Error, req.ErrorDescription))
			return
		}

		var currentSessionId string
		if raw := c.Cookie(schema.AuthSessionCookieName); len(raw) > 0 {
			currentSessionId = string(raw)
		}

		// handle callback logic
		resp, err := s.authCallbackHandler.Handle(ctx, &authapp.CallbackHandleCommand{
			State:            req.State,
			Code:             req.Code,
			ProviderType:     providerType,
			Device:           entity.DeviceTypeWeb,
			CurrentSessionId: currentSessionId,
		})
		if err != nil {
			http.ErrResp(c, err)
			return
		}

		// set user session cookie
		c.SetCookie(schema.AuthSessionCookieName,
			resp.SessionId,
			schema.AuthSessionCookieTTL,
			"/",
			"",
			protocol.CookieSameSiteLaxMode,
			true,
			true,
		)

		// set user id cookie (readable by frontend, do not trust for authz)
		c.SetCookie(schema.AuthUserIdCookieName,
			resp.UserId,
			schema.AuthSessionCookieTTL,
			"/",
			"",
			protocol.CookieSameSiteLaxMode,
			true,
			false,
		)

		// clear state cookie
		c.SetCookie(schema.AuthLoginStateCookieName, "", -1, "/", "", protocol.CookieSameSiteLaxMode, true, true)

		returnTo := resp.ReturnTo
		if returnTo == "" || !schema.IsSafeReturnTo(returnTo, conf.NotelmGlobal().Auth.ReturnToAllowOrigins) {
			returnTo = "/"
		}
		c.Redirect(stdhttp.StatusFound, []byte(returnTo))
	}
}
