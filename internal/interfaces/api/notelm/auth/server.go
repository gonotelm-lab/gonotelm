package auth

import (
	authapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/auth"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
)

// Deps 认证接口所需依赖，由外层 notelm.ServerDeps 裁剪后传入
type Deps struct {
	ProviderRegistry        identrepo.ProviderRegistry
	OAuthStateRepo          identrepo.OAuthStateRepository
	UserService             *identityservice.UserService
	SignupService           *identityservice.SignupService
	PasswordResetService    *identityservice.PasswordResetService
	UserSessionRepo         identrepo.UserSessionRepository
	EmailVerificationMailer identrepo.EmailVerificationMailer
	PasswordResetMailer     identrepo.PasswordResetMailer
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
	authResendMailHandler              *authapp.ResendMailHandler
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
			deps.EmailVerificationMailer,
		),
		authPrefetchHandler:   authapp.NewPrefetchHandler(),
		authVerifyHandler:     authapp.NewVerifyHandler(deps.SignupService, deps.UserService, deps.UserSessionRepo),
		authResendMailHandler: authapp.NewResendMailHandler(deps.SignupService, deps.EmailVerificationMailer),
		authPasswordResetHandler: authapp.NewPasswordResetHandler(
			deps.PasswordResetService,
			deps.UserSessionRepo,
			deps.PasswordResetMailer,
		),
		authPasswordLoginHandler:           authapp.NewPasswordLoginHandler(deps.UserService, deps.UserSessionRepo),
		authSetPasswordHandler:             authapp.NewSetPasswordHandler(deps.UserService, deps.UserSessionRepo),
		authBindIdentityHandler:            authapp.NewBindIdentityHandler(deps.ProviderRegistry, deps.OAuthStateRepo, deps.UserService),
		authUnbindIdentityHandler:          authapp.NewUnbindIdentityHandler(deps.UserService),
		authListIdentitiesHandler:          authapp.NewListIdentitiesHandler(deps.UserService),
		authListSupportedIdentitiesHandler: authapp.NewListSupportedIdentitiesHandler(),
	}
}
