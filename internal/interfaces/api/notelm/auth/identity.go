package auth

import (
	"context"

	authapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/auth"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
)

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

	resp, err := s.authBindIdentityHandler.Handle(ctx, &authapp.BindIdentityCommand{
		ProviderType: entity.ProviderType(req.Provider),
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

	if err := s.authUnbindIdentityHandler.Handle(ctx, &authapp.UnbindIdentityCommand{
		BindId:       req.BindId,
		ProviderType: entity.ProviderType(req.Provider),
	}); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkRespNoContent(c)
}
