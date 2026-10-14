package auth

import (
	"context"

	authapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/auth"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
)

// AuthPrefetch 注册页与重置页共用的表单元数据：密码复杂度要求
func (s *Server) AuthPrefetch(ctx context.Context, c *app.RequestContext) {
	resp := s.authPrefetchHandler.Handle()

	http.OkResp(c, schema.AuthPrefetchResponse{
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
		Ticket:   s.signupTicket(c),
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

	// 当前浏览器没有有效票据时才回传 ticket：首次注册，或上次发信失败后重试
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

	if _, err := s.authResendMailHandler.Handle(ctx, &authapp.ResendCommand{Email: req.Email}); err != nil {
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
