package auth

import (
	"context"
	stdhttp "net/http"

	authapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/auth"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	domainerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
)

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

	c.SetCookie(schema.AuthOAuthStateCookieName,
		resp.LoginInfo.State,
		schema.AuthOAuthStateCookieTTL,
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

	http.OkResp(c, schema.AuthProvidersResponse{
		Providers:       providers,
		PasswordEnabled: true,
	})
}

func (s *Server) AuthCallback(providerType entity.ProviderType) func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		var req schema.AuthCallbackRequest
		if err := c.BindAndValidate(&req); err != nil {
			// 回调地址是浏览器整页导航，失败一律 302 回前端，不能回 JSON
			s.redirectCallbackError(ctx, c, providerType, nil, err)
			return
		}

		// check if is error
		if req.IsError() {
			// provider 侧报错时 state 还在，用它决定跳到登录页还是账号设置页
			s.redirectCallbackError(ctx, c, providerType,
				s.authCallbackHandler.PeekState(ctx, req.State), domainerr.ErrIDPError)
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
			CurrentSessionId: currentSessionId,
		})
		if err != nil {
			s.redirectCallbackError(ctx, c, providerType, resp, err)
			return
		}

		// 只有登录流程创建新会话，绑定流程继续用当前会话
		if !resp.Mode.IsBindMode() {
			s.setSessionCookies(c, resp.SessionId, resp.UserId)
		}

		// clear state cookie
		c.SetCookie(schema.AuthOAuthStateCookieName, "", -1, "/", "", protocol.CookieSameSiteLaxMode, true, true)

		returnTo := resp.ReturnTo
		if returnTo == "" || !schema.IsSafeReturnTo(returnTo, conf.NotelmGlobal().Auth.ReturnToAllowOrigins) {
			returnTo = "/"
		}
		c.Redirect(stdhttp.StatusFound, []byte(returnTo))
	}
}
