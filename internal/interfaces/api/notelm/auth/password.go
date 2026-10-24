package auth

import (
	"context"

	authapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/auth"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
)

func (s *Server) AuthSetPassword(ctx context.Context, c *app.RequestContext) {
	var req schema.SetPasswordRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.authSetPasswordHandler.Handle(ctx, &authapp.SetPasswordCommand{
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

// AuthPasswordReset 发起找回密码：账号存在才发送邮件，响应跟账号存不存在无关
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
