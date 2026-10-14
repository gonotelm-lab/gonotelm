package auth

import (
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/http/middleware"

	"github.com/cloudwego/hertz/pkg/route"
)

// RegisterPublicRoutes 挂不需要登录的认证接口，由外层挂在 /api/v1 上
func (s *Server) RegisterPublicRoutes(g *route.RouterGroup) {
	// 认证响应带会话标识与账号邮箱，且登录/注册/验证都是可变更状态的操作，一律禁止缓存
	authGroup := g.Group("/auth", middleware.NoStore())
	{
		// GET /api/v1/auth/login 发起三方登录，302 跳 IdP
		authGroup.GET("/login", s.AuthLogin)
		// POST /api/v1/auth/logout
		authGroup.POST("/logout", s.AuthLogout)
		// GET /api/v1/auth/providers
		authGroup.GET("/providers", s.AuthProviders)
		// GET /api/v1/auth/prefetch 注册页与重置页共用的表单元数据，含密码复杂度要求
		authGroup.GET("/prefetch", s.AuthPrefetch)

		emailGroup := authGroup.Group("/email")
		{
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

// RegisterAuthedRoutes 挂需要登录的认证接口，由外层挂在鉴权组下
func (s *Server) RegisterAuthedRoutes(g *route.RouterGroup) {
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
