package auth

import (
	authapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/auth"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
)

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
