package notelm

import (
	"context"
	stdhttp "net/http"

	authapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/auth"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	domainerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/http/middleware"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route"
)

func (s *Server) registerAuthRoutes(g *route.RouterGroup) {
	// 认证响应带会话标识与账号邮箱，且登录/注册/验证都是可变更状态的操作，一律禁止缓存
	authGroup := g.Group("/auth", middleware.NoStore())
	{
		// GET /api/v1/auth/login 发起三方登录，302 跳 IdP
		authGroup.GET("/login", s.AuthLogin)
		// POST /api/v1/auth/logout
		authGroup.POST("/logout", s.AuthLogout)
		// GET /api/v1/auth/providers
		authGroup.GET("/providers", s.AuthProviders)

		emailGroup := authGroup.Group("/email")
		{
			// GET /api/v1/auth/email/register/prefetch 注册前元数据，含密码复杂度要求
			emailGroup.GET("/register/prefetch", s.AuthEmailRegisterPrefetch)
			// POST /api/v1/auth/email/register
			emailGroup.POST("/register", s.AuthEmailRegister)
			// POST /api/v1/auth/email/login 邮箱密码登录
			emailGroup.POST("/login", s.AuthEmailLogin)
			// POST /api/v1/auth/email/verify/code 验证码
			emailGroup.POST("/verify/code", s.AuthEmailVerifyCode)
			// POST /api/v1/auth/email/resend
			emailGroup.POST("/resend", s.AuthEmailResend)
		}

		passwordGroup := authGroup.Group("/password")
		{
			// GET /api/v1/auth/password/reset/prefetch 重置页元数据，含密码复杂度要求
			passwordGroup.GET("/reset/prefetch", s.AuthPasswordResetPrefetch)
			// POST /api/v1/auth/password/reset 发起找回密码，账号不存在也返回 204
			passwordGroup.POST("/reset", s.AuthPasswordReset)
			// POST /api/v1/auth/password/reset/verify 邮件链接确认
			passwordGroup.POST("/reset/verify", s.AuthPasswordResetConfirmLink)
		}

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

// registerAuthBindRoutes 挂在鉴权组下，身份与设置密码相关接口都要求已登录
func (s *Server) registerAuthBindRoutes(g *route.RouterGroup) {
	// 同上：响应里带账号身份列表，不能进任何缓存
	authGroup := g.Group("/auth", middleware.NoStore())
	{
		// GET /api/v1/auth/identities 当前账号已绑定的三方身份
		authGroup.GET("/identities", s.AuthIdentities)
		// GET /api/v1/auth/supported-identities 代码支持的身份类型目录
		authGroup.GET("/supported-identities", s.AuthSupportedIdentities)
		// PUT /api/v1/auth/password
		authGroup.PUT("/password", s.AuthSetPassword)
		// POST /api/v1/auth/identities/bind
		authGroup.POST("/identities/bind", s.AuthBind)
		// POST /api/v1/auth/identities/unbind
		authGroup.POST("/identities/unbind", s.AuthUnbind)
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

// AuthEmailRegisterPrefetch 注册前元数据：密码复杂度要求
func (s *Server) AuthEmailRegisterPrefetch(ctx context.Context, c *app.RequestContext) {
	resp := s.authRegisterPrefetchHandler.Handle()

	http.OkResp(c, schema.EmailRegisterPrefetchResponse{
		RequireComplexPassword: resp.RequireComplexPassword,
	})
}

func (s *Server) AuthEmailRegister(ctx context.Context, c *app.RequestContext) {
	var req schema.EmailRegisterRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.authRegisterHandler.Handle(ctx, &authapp.RegisterCommand{
		Email:    req.Email,
		Password: req.Password,
		Device:   schema.DeviceOrWeb(req.LoginFrom),
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	// 免验证：账号与会话已就绪，不返回注册 ticket
	if resp.Session != nil {
		c.SetCookie(schema.SignupTicketCookieName, "", -1, "/", "", protocol.CookieSameSiteLaxMode, true, true)
		s.setSessionCookies(c, resp.Session.SessionId, resp.Session.UserId)
		http.OkResp(c, schema.EmailRegisterResponse{
			EmailVerified: true,
			UserId:        resp.Session.UserId,
		})

		return
	}

	// 只有首次注册返回 ticket，重复注册沿用旧 ticket，不重置 cookie
	if resp.Ticket != "" {
		c.SetCookie(schema.SignupTicketCookieName,
			resp.Ticket,
			schema.SignupTicketCookieTTL,
			"/",
			"",
			protocol.CookieSameSiteLaxMode,
			true,
			true,
		)
	}

	http.OkResp(c, schema.EmailRegisterResponse{EmailVerified: false})
}

func (s *Server) AuthEmailVerifyCode(ctx context.Context, c *app.RequestContext) {
	var req schema.EmailVerifyCodeRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.authVerifyHandler.VerifyCode(ctx, &authapp.VerifyCommand{
		Email:  req.Email,
		Code:   req.Code,
		Device: schema.DeviceOrWeb(req.LoginFrom),
		Ticket: s.signupTicket(c),
	})

	s.finishSession(c, resp, err)
}

func (s *Server) AuthEmailResend(ctx context.Context, c *app.RequestContext) {
	var req schema.EmailResendRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	if _, err := s.authResendHandler.Handle(ctx, &authapp.ResendCommand{Email: req.Email}); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkRespNoContent(c)
}

func (s *Server) AuthEmailLogin(ctx context.Context, c *app.RequestContext) {
	var req schema.PasswordLoginRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.authPasswordLoginHandler.Handle(ctx, &authapp.PasswordLoginCommand{
		Email:            req.Email,
		Password:         req.Password,
		Device:           schema.DeviceOrWeb(req.LoginFrom),
		CurrentSessionId: string(c.Cookie(schema.AuthSessionCookieName)),
	})

	s.finishSession(c, resp, err)
}

func (s *Server) AuthSetPassword(ctx context.Context, c *app.RequestContext) {
	var req schema.SetPasswordRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.authSetPasswordHandler.Handle(ctx, &authapp.SetPasswordCommand{
		UserId:      pkgcontext.GetUserId(ctx),
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	// 全部会话已作废：清掉当前浏览器的 cookie，让前端引导重新登录
	c.SetCookie(schema.AuthSessionCookieName, "", -1, "/", "", protocol.CookieSameSiteLaxMode, true, true)
	c.SetCookie(schema.AuthUserIdCookieName, "", -1, "/", "", protocol.CookieSameSiteLaxMode, true, false)

	http.OkResp(c, schema.SetPasswordResponse{ReloginRequired: resp.ReloginRequired})
}

// AuthPasswordResetPrefetch 重置页元数据：密码复杂度要求
// 复用注册页那份策略，两处校验的是同一份配置
func (s *Server) AuthPasswordResetPrefetch(ctx context.Context, c *app.RequestContext) {
	resp := s.authRegisterPrefetchHandler.Handle()

	http.OkResp(c, schema.PasswordResetPrefetchResponse{
		RequireComplexPassword: resp.RequireComplexPassword,
	})
}

// AuthPasswordReset 发起找回密码：账号存在才发信，响应跟账号存不存在无关
func (s *Server) AuthPasswordReset(ctx context.Context, c *app.RequestContext) {
	var req schema.PasswordResetRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	if _, err := s.authPasswordResetHandler.Handle(ctx, &authapp.PasswordResetCommand{
		Email: req.Email,
	}); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkRespNoContent(c)
}

// AuthPasswordResetConfirmLink 邮件链接确认：token 由前端从 URL 片段读出后提交
// 成功返回 204，新密码要用户回登录页自己登一次
func (s *Server) AuthPasswordResetConfirmLink(ctx context.Context, c *app.RequestContext) {
	var req schema.PasswordResetConfirmRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	if err := s.authPasswordResetHandler.ConfirmLink(ctx, &authapp.PasswordResetConfirmCommand{
		Token:       req.Token,
		NewPassword: req.NewPassword,
	}); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkRespNoContent(c)
}

func (s *Server) AuthIdentities(ctx context.Context, c *app.RequestContext) {
	resp, err := s.authListIdentitiesHandler.Handle(ctx)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	identities := make([]schema.AuthIdentity, 0, len(resp.Identities))
	for _, identity := range resp.Identities {
		identities = append(identities, schema.AuthIdentity{
			BindId:                identity.Id.String(),
			Provider:              identity.Provider.String(),
			ProviderEmail:         identity.ProviderEmail,
			ProviderEmailVerified: identity.ProviderEmailVerified,
			BoundAt:               identity.BoundAt.Value(),
			LastLoginAt:           identity.LastLoginAt.Value(),
		})
	}

	http.OkResp(c, schema.AuthIdentitiesResponse{Identities: identities})
}

func (s *Server) AuthSupportedIdentities(ctx context.Context, c *app.RequestContext) {
	resp, err := s.authListSupportedIdentitiesHandler.Handle(ctx)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	identities := make([]schema.AuthSupportedIdentity, 0, len(resp.Identities))
	for _, identity := range resp.Identities {
		identities = append(identities, schema.AuthSupportedIdentity{Provider: identity.Provider.String()})
	}

	http.OkResp(c, schema.AuthSupportedIdentitiesResponse{Identities: identities})
}

func (s *Server) AuthBind(ctx context.Context, c *app.RequestContext) {
	var req schema.BindProviderRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	allowedOrigins := conf.NotelmGlobal().Auth.ReturnToAllowOrigins
	if !schema.IsSafeReturnTo(req.ReturnTo, allowedOrigins) {
		http.ErrResp(c, errors.ErrParams.Msgf("invalid return_to: %s", req.ReturnTo))
		return
	}

	resp, err := s.authBindHandler.Handle(ctx, &authapp.BindCommand{
		ProviderType: entity.ProviderType(req.Provider),
		UserId:       pkgcontext.GetUserId(ctx),
		SessionId:    string(c.Cookie(schema.AuthSessionCookieName)),
		ReturnTo:     req.ReturnTo,
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	// 已绑在当前账号上：不发 state，不返回授权地址
	if resp.AlreadyBound {
		http.OkResp(c, schema.BindProviderResponse{AlreadyBound: true})
		return
	}

	// state cookie 由后端写入并在回调时校验，跳转交给前端
	c.SetCookie(schema.AuthOAuthStateCookieName,
		resp.LoginInfo.State,
		schema.AuthOAuthStateCookieTTL,
		"/",
		"",
		protocol.CookieSameSiteLaxMode,
		true,
		true,
	)

	http.OkResp(c, schema.BindProviderResponse{AuthorizeUrl: resp.LoginInfo.URL})
}

func (s *Server) AuthUnbind(ctx context.Context, c *app.RequestContext) {
	var req schema.UnbindIdentityRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	if err := s.authUnbindHandler.Handle(ctx, &authapp.UnbindCommand{
		BindId:       req.BindId,
		ProviderType: entity.ProviderType(req.Provider),
		UserId:       pkgcontext.GetUserId(ctx),
	}); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkRespNoContent(c)
}

// finishSession 登录/验证成功统一处理：写会话 cookie，清注册 ticket，返回账号标识
func (s *Server) finishSession(c *app.RequestContext, resp *authapp.SessionResult, err error) {
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	s.setSessionCookies(c, resp.SessionId, resp.UserId)
	c.SetCookie(schema.SignupTicketCookieName, "", -1, "/", "", protocol.CookieSameSiteLaxMode, true, true)

	http.OkResp(c, schema.AuthSessionResponse{UserId: resp.UserId})
}

func (s *Server) setSessionCookies(c *app.RequestContext, sessionId, userId string) {
	c.SetCookie(schema.AuthSessionCookieName,
		sessionId,
		schema.AuthSessionCookieTTL,
		"/",
		"",
		protocol.CookieSameSiteLaxMode,
		true,
		true,
	)

	// set user id cookie (readable by frontend, do not trust for authz)
	c.SetCookie(schema.AuthUserIdCookieName,
		userId,
		schema.AuthSessionCookieTTL,
		"/",
		"",
		protocol.CookieSameSiteLaxMode,
		true,
		false,
	)
}

func (s *Server) signupTicket(c *app.RequestContext) string {
	return string(c.Cookie(schema.SignupTicketCookieName))
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
