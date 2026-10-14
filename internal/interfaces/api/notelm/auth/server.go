// Package auth 提供 /api/v1/auth 下的认证接口：三方登录、邮箱注册/验证、密码登录、找回密码、身份绑定。
// 路由注册见 routes.go，各功能按主题拆在 oauth/email/password/identity 文件里。
package auth

import (
	authapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/auth"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
)

// Deps 认证接口所需依赖，由外层 notelm.ServerDeps 裁剪后传入
type Deps struct {
	ProviderRegistry     identrepo.ProviderRegistry
	OAuthStateRepo       identrepo.OAuthStateRepository
	UserService          *identityservice.UserService
	SignupService        *identityservice.SignupService
	PasswordResetService *identityservice.PasswordResetService
	UserSessionRepo      identrepo.UserSessionRepository
	Mailer               adapter.Mailer
}

// Server 认证接口的处理器集合
type Server struct {
	authLoginHandler     *authapp.LoginHandler
	authCallbackHandler  *authapp.CallbackHandler
	authProvidersHandler *authapp.ProvidersHandler
	authHandler          *authapp.AuthHandler

	authRegisterHandler                *authapp.RegisterHandler
	authPrefetchHandler                *authapp.PrefetchHandler
	authVerifyHandler                  *authapp.VerifyHandler
	authResendHandler                  *authapp.ResendHandler
	authPasswordResetHandler           *authapp.PasswordResetHandler
	authPasswordLoginHandler           *authapp.PasswordLoginHandler
	authSetPasswordHandler             *authapp.SetPasswordHandler
	authBindIdentityHandler            *authapp.BindIdentityHandler
	authUnbindIdentityHandler          *authapp.UnbindIdentityHandler
	authListIdentitiesHandler          *authapp.ListIdentitiesHandler
	authListSupportedIdentitiesHandler *authapp.ListSupportedIdentitiesHandler
}

func NewServer(deps Deps) *Server {
	return &Server{
		authLoginHandler:     authapp.NewLoginHandler(deps.ProviderRegistry, deps.OAuthStateRepo, deps.UserService, deps.UserSessionRepo),
		authCallbackHandler:  authapp.NewCallbackHandler(deps.ProviderRegistry, deps.OAuthStateRepo, deps.UserService, deps.UserSessionRepo),
		authProvidersHandler: authapp.NewProvidersHandler(deps.ProviderRegistry),
		authHandler:          authapp.NewAuthHandler(deps.UserService, deps.UserSessionRepo),

		authRegisterHandler: authapp.NewRegisterHandler(
			deps.SignupService,
			deps.UserService,
			deps.UserSessionRepo,
			deps.Mailer,
		),
		authPrefetchHandler: authapp.NewPrefetchHandler(),
		authVerifyHandler:   authapp.NewVerifyHandler(deps.SignupService, deps.UserService, deps.UserSessionRepo),
		authResendHandler:   authapp.NewResendHandler(deps.SignupService, deps.Mailer),
		authPasswordResetHandler: authapp.NewPasswordResetHandler(
			deps.PasswordResetService,
			deps.UserSessionRepo,
			deps.Mailer,
		),
		authPasswordLoginHandler:           authapp.NewPasswordLoginHandler(deps.UserService, deps.UserSessionRepo),
		authSetPasswordHandler:             authapp.NewSetPasswordHandler(deps.UserService, deps.UserSessionRepo),
		authBindIdentityHandler:            authapp.NewBindIdentityHandler(deps.ProviderRegistry, deps.OAuthStateRepo, deps.UserService),
		authUnbindIdentityHandler:          authapp.NewUnbindIdentityHandler(deps.UserService),
		authListIdentitiesHandler:          authapp.NewListIdentitiesHandler(deps.UserService),
		authListSupportedIdentitiesHandler: authapp.NewListSupportedIdentitiesHandler(),
	}
}
