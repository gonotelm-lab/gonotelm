package schema

import (
	"net/url"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/cloudwego/hertz/pkg/protocol"
)

const (
	AuthLoginStateCookieName = "_gnlm_login_state"
	AuthLoginStateCookieTTL  = 600 // 10 minutes

	AuthSessionCookieName = "gnlm_user_sid"
	// cookie 生命周期对齐会话的绝对上限;会话本身可能因闲置提前失效。
	AuthSessionCookieTTL = int(entity.UserSessionMaxAge / time.Second)

	AuthUserIdCookieName = "gnlm_user_id"

	CSRFCookieName = "gnlm_csrf"
	CSRFHeaderName = "X-CSRF-Token"
)

type AuthLoginRequest struct {
	LoginProvider string `query:"login_provider,required"`
	LoginFrom     string `query:"login_from,required"`
	ReturnTo      string `query:"return_to,omitempty"` // in query
}

func (req *AuthLoginRequest) Validate() error {
	if req.LoginProvider == "" {
		return errors.ErrParams.Msg("login provider is required")
	}

	if req.LoginFrom == "" {
		return errors.ErrParams.Msg("login from is required")
	}

	if !entity.CheckDeviceType(req.LoginFrom) {
		return errors.ErrParams.Msgf("invalid login from: %s", req.LoginFrom)
	}

	if !entity.CheckProviderType(req.LoginProvider) {
		return errors.ErrParams.Msgf("invalid login provider: %s", req.LoginProvider)
	}

	return nil
}

// IsSafeReturnTo 允许站内相对路径，或来源在 allowedOrigins 白名单内的绝对 URL，防止开放重定向
func IsSafeReturnTo(raw string, allowedOrigins []string) bool {
	if raw == "" {
		return true
	}

	if strings.HasPrefix(raw, "/") {
		if strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "/\\") || strings.ContainsAny(raw, "\\\r\n") {
			return false
		}

		u, err := url.Parse(raw)
		return err == nil && u.Scheme == "" && u.Host == ""
	}

	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}

	origin := u.Scheme + "://" + u.Host
	for _, allowed := range allowedOrigins {
		if strings.EqualFold(strings.TrimRight(strings.TrimSpace(allowed), "/"), origin) {
			return true
		}
	}

	return false
}

type AuthLogoutRequest struct{}

type AuthProvider struct {
	Name string `json:"name"`
}

type AuthProvidersResponse struct {
	Providers []AuthProvider `json:"providers"`
}

type AuthCallbackRequest struct {
	State string `query:"state,required"`

	// Success response
	Code string `query:"code"`

	// Error response
	Error            string `query:"error,omitempty"`
	ErrorDescription string `query:"error_description,omitempty"`
	ErrorUri         string `query:"error_uri,omitempty"`

	// Extra fields, differs for different providers
	Iss string `query:"iss"`
}

func (r *AuthCallbackRequest) IsError() bool {
	return r.Error != "" || r.ErrorDescription != ""
}

func (r *AuthCallbackRequest) Validate(req *protocol.Request) error {
	if r.State == "" {
		return errors.ErrUnauthorized.Msg("state is required")
	}

	stateCookie := req.Header.Cookie(AuthLoginStateCookieName)
	if string(stateCookie) != r.State {
		return errors.ErrUnauthorized.Msg("cookie state mismatch")
	}

	return nil
}
