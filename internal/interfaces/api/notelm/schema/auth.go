package schema

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/cloudwego/hertz/pkg/protocol"
)

const (
	AuthOAuthStateCookieName = "_gnlm_oauth_state"
	AuthOAuthStateCookieTTL  = 600 // 10 minutes

	// SignupTicketCookieName 注册 ticket：只有发起注册的浏览器完成验证才写入密码
	SignupTicketCookieName = "_gnlm_signup_ticket"
	SignupTicketCookieTTL  = 24 * 60 * 60

	AuthSessionCookieName = "gnlm_user_sid"
	// cookie 生命周期对齐会话的绝对上限;会话本身可能因闲置提前失效。
	AuthSessionCookieTTL = int(entity.UserSessionMaxAge / time.Second)

	AuthUserIdCookieName = "gnlm_user_id"

	CSRFCookieName = "gnlm_csrf"
	CSRFHeaderName = "X-CSRF-Token"
)

// AuthSessionResponse 登录/验证成功后返回的账号标识
type AuthSessionResponse struct {
	UserId string `json:"user_id"`
}

// DeviceOrWeb 缺省的登录来源设备
func DeviceOrWeb(raw string) entity.DeviceType {
	if raw == "" {
		return entity.DeviceTypeWeb
	}

	return entity.DeviceType(raw)
}

// EmailRegisterResponse 注册结果，EmailVerified=true 时账号已建好并创建会话
// false 时账号未建立，等待验证邮件里的链接或验证码
type EmailRegisterResponse struct {
	EmailVerified bool   `json:"email_verified"`
	UserId        string `json:"user_id,omitempty"`
}

// AuthPrefetchResponse 注册页与重置页共用的表单元数据，客户端据此即时校验表单
type AuthPrefetchResponse struct {
	RequireComplexPassword bool `json:"require_complex_password"`
}

type EmailRegisterRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	LoginFrom string `json:"login_from,omitempty"`
}

func (req *EmailRegisterRequest) Validate() error {
	if err := entity.ValidateEmail(req.Email); err != nil {
		return identerr.ErrEmailInvalid
	}
	if req.Password == "" {
		return errors.ErrParams.Msg("password is required")
	}
	if req.LoginFrom != "" && !entity.CheckDeviceType(req.LoginFrom) {
		return errors.ErrParams.Msgf("invalid login from: %s", req.LoginFrom)
	}

	return nil
}

type EmailVerifyCodeRequest struct {
	Email     string `json:"email"`
	Code      string `json:"code"`
	LoginFrom string `json:"login_from,omitempty"`
}

func (req *EmailVerifyCodeRequest) Validate() error {
	if err := entity.ValidateEmail(req.Email); err != nil {
		return identerr.ErrEmailInvalid
	}
	if req.Code == "" {
		return errors.ErrParams.Msg("code is required")
	}
	if req.LoginFrom != "" && !entity.CheckDeviceType(req.LoginFrom) {
		return errors.ErrParams.Msgf("invalid login from: %s", req.LoginFrom)
	}

	return nil
}

type EmailResendRequest struct {
	Email string `json:"email"`
}

func (req *EmailResendRequest) Validate() error {
	if err := entity.ValidateEmail(req.Email); err != nil {
		return identerr.ErrEmailInvalid
	}

	return nil
}

type PasswordLoginRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	LoginFrom string `json:"login_from,omitempty"`
}

func (req *PasswordLoginRequest) Validate() error {
	if req.Email == "" {
		return errors.ErrParams.Msg("email is required")
	}
	if req.Password == "" {
		return errors.ErrParams.Msg("password is required")
	}
	if req.LoginFrom != "" && !entity.CheckDeviceType(req.LoginFrom) {
		return errors.ErrParams.Msgf("invalid login from: %s", req.LoginFrom)
	}

	return nil
}

type SetPasswordRequest struct {
	OldPassword string `json:"old_password,omitempty"`
	NewPassword string `json:"new_password"`
}

func (req *SetPasswordRequest) Validate() error {
	if req.NewPassword == "" {
		return errors.ErrParams.Msg("new_password is required")
	}

	return nil
}

// SetPasswordResponse 改密码作废该账号全部会话（含当前），前端据此引导重新登录
type SetPasswordResponse struct {
	ReloginRequired bool `json:"relogin_required"`
}

// PasswordResetRequest 发起找回密码
// 账号不存在时接口也返回成功，前端没法据此判断邮箱是否注册
type PasswordResetRequest struct {
	Email string `json:"email"`
}

func (req *PasswordResetRequest) Validate() error {
	if err := entity.ValidateEmail(entity.NormalizeEmail(req.Email)); err != nil {
		return identerr.ErrEmailInvalid
	}

	return nil
}

// PasswordResetConfirmRequest 链接确认：token 取自邮件里重置链接的 URL 片段
type PasswordResetConfirmRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (req *PasswordResetConfirmRequest) Validate() error {
	if req.Token == "" {
		return errors.ErrParams.Msg("token is required")
	}
	if req.NewPassword == "" {
		return errors.ErrParams.Msg("new_password is required")
	}

	return nil
}

type BindProviderRequest struct {
	Provider string `json:"provider"`
	ReturnTo string `json:"return_to,omitempty"`
}

func (req *BindProviderRequest) Validate() error {
	if req.Provider == "" {
		return errors.ErrParams.Msg("provider is required")
	}
	if !entity.CheckProviderType(req.Provider) {
		return errors.ErrParams.Msgf("invalid provider: %s", req.Provider)
	}

	return nil
}

// UnbindIdentityRequest 解绑三方身份：bind_id 即 /auth/identities 返回的 bind_id
// provider 交叉校验，必须与该条绑定记录的 provider 一致
type UnbindIdentityRequest struct {
	BindId   valobj.Id `json:"bind_id"`
	Provider string    `json:"provider"`
}

func (req *UnbindIdentityRequest) Validate() error {
	if req.BindId.IsZero() {
		return errors.ErrParams.Msg("bind_id is required")
	}
	if req.Provider == "" {
		return errors.ErrParams.Msg("provider is required")
	}
	if !entity.CheckProviderType(req.Provider) {
		return errors.ErrParams.Msgf("invalid provider: %s", req.Provider)
	}

	return nil
}

// BindProviderResponse 返回 IdP 授权地址，由前端自行跳转，后端不 302
// AlreadyBound=true 时 authorize_url 为空，该 provider 已绑在当前账号上
type BindProviderResponse struct {
	AuthorizeUrl string `json:"authorize_url"`
	AlreadyBound bool   `json:"already_bound"`
}

// 已绑定的三方账号
type AuthIdentity struct {
	BindId                string `json:"bind_id"`
	Provider              string `json:"provider"`
	ProviderEmail         string `json:"provider_email"`
	ProviderEmailVerified bool   `json:"provider_email_verified"`
	BoundAt               int64  `json:"bound_at"`      // unix ms
	LastLoginAt           int64  `json:"last_login_at"` // unix ms
}

type AuthIdentitiesResponse struct {
	Identities []AuthIdentity `json:"identities"`
}

// AuthSupportedIdentity 代码支持的身份类型，不含是否配置凭证（见 /auth/providers）
type AuthSupportedIdentity struct {
	Provider string `json:"provider"`
}

type AuthSupportedIdentitiesResponse struct {
	Identities []AuthSupportedIdentity `json:"identities"`
}

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
	// PasswordEnabled 前端据此决定是否渲染邮箱+密码表单
	PasswordEnabled bool `json:"password_enabled"`
}

type AuthCallbackRequest struct {
	State string `query:"state"`

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
	// 用 domain 错误码，回调失败要 302 并把 reason 带给前端
	if r.State == "" {
		return identerr.ErrOAuthStateNotFound
	}

	stateCookie := req.Header.Cookie(AuthOAuthStateCookieName)
	if string(stateCookie) != r.State {
		return identerr.ErrOAuthStateMismatch
	}

	return nil
}

// ── 打印脱敏 ──────────────────────────────────────────────────────────────
//
// 下面这些 DTO 带密码或一次性凭证，实现 fmt.Stringer 是为了万一有人 %+v
// 打整个请求体（handler、中间件、错误日志都可能），不至于把密码和能直接换
// 会话的凭证写进日志。
//
// String 都用值接收者：handler 传的是指针，但 %+v 打在值上也得命中。
// 不带凭证的 DTO 不实现 String，它们的默认输出本来就安全。

// redactedValue 密码与一次性凭证的打印占位符
const redactedValue = "[REDACTED]"

func (req EmailRegisterRequest) String() string {
	return fmt.Sprintf("EmailRegisterRequest{Email:%q, Password:%s, LoginFrom:%q}",
		req.Email, redactedValue, req.LoginFrom)
}

func (req EmailVerifyCodeRequest) String() string {
	return fmt.Sprintf("EmailVerifyCodeRequest{Email:%q, Code:%s, LoginFrom:%q}",
		req.Email, redactedValue, req.LoginFrom)
}

func (req PasswordLoginRequest) String() string {
	return fmt.Sprintf("PasswordLoginRequest{Email:%q, Password:%s, LoginFrom:%q}",
		req.Email, redactedValue, req.LoginFrom)
}

func (req SetPasswordRequest) String() string {
	return fmt.Sprintf("SetPasswordRequest{OldPassword:%s, NewPassword:%s}", redactedValue, redactedValue)
}

func (req PasswordResetConfirmRequest) String() string {
	return fmt.Sprintf("PasswordResetConfirmRequest{Token:%s, NewPassword:%s}", redactedValue, redactedValue)
}

func (r AuthCallbackRequest) String() string {
	return fmt.Sprintf("AuthCallbackRequest{State:%s, Code:%s, Error:%q, ErrorDescription:%q, ErrorUri:%q, Iss:%q}",
		redactedValue, redactedValue, r.Error, r.ErrorDescription, r.ErrorUri, r.Iss)
}
