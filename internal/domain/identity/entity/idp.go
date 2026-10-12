package entity

import "context"

type ProviderType string

const (
	ProviderTypeGithub ProviderType = "github"
	ProviderTypeGoogle ProviderType = "google"
)

var supportedProviderTypes = []ProviderType{
	ProviderTypeGithub,
	ProviderTypeGoogle,
}

var validProviderTypes = func() map[ProviderType]bool {
	valid := make(map[ProviderType]bool, len(supportedProviderTypes))
	for _, provider := range supportedProviderTypes {
		valid[provider] = true
	}

	return valid
}()

// SupportedProviderTypes 返回代码支持的身份类型
func SupportedProviderTypes() []ProviderType {
	return append([]ProviderType(nil), supportedProviderTypes...)
}

func (t ProviderType) String() string {
	return string(t)
}

func (t ProviderType) Iss() string {
	switch t {
	case ProviderTypeGithub:
		return "https://github.com/login/oauth"
	case ProviderTypeGoogle:
		return "https://accounts.google.com"
	default:
		return ""
	}
}

// ProviderLoginMode 区分"用三方登录"与"把三方身份绑到当前账号"。
type ProviderLoginMode string

const (
	ProviderLoginModeLogin ProviderLoginMode = "login"
	ProviderLoginModeBind  ProviderLoginMode = "bind"
)

func (m ProviderLoginMode) IsLoginMode() bool {
	return m == ProviderLoginModeLogin
}

func (m ProviderLoginMode) IsBindMode() bool {
	return m == ProviderLoginModeBind
}

type ProviderOAuthInfo struct {
	URL                 string       // full login url with query params to redirect to with http status code 302
	ClientId            string       // client_id
	State               string       // random state
	Nonce               string       // random nonce, OIDC providers echo it back in the id_token
	CodeVerifier        string       // random code verifier
	CodeChallenge       string       // usually base64url(sha256(codeVerifier))
	CodeChallengeMethod string       // code challenge method, always "S256"
	ProviderType        ProviderType // provider type
	ReturnTo            string       // return to url to redirect after login
	Device              DeviceType   // device the login started from, carried through to the session
	Mode                ProviderLoginMode
	// SessionId 绑定模式下发起绑定的会话，回调必须来自同一个会话
	SessionId string
}

func (info *ProviderOAuthInfo) ToOAuthState() *OAuthState {
	return &OAuthState{
		State:               info.State,
		Nonce:               info.Nonce,
		CodeVerifier:        info.CodeVerifier,
		CodeChallenge:       info.CodeChallenge,
		CodeChallengeMethod: info.CodeChallengeMethod,
		ProviderType:        info.ProviderType,
		ReturnTo:            info.ReturnTo,
		Device:              info.Device,
		Mode:                info.Mode,
		SessionId:           info.SessionId,
	}
}

// OAuthState OAuth 握手期间的一次性状态：state/nonce/code_verifier 等，短 TTL 存储。
type OAuthState struct {
	State               string
	Nonce               string
	CodeVerifier        string
	CodeChallenge       string
	CodeChallengeMethod string
	ReturnTo            string
	ProviderType        ProviderType
	Device              DeviceType
	Mode                ProviderLoginMode
	SessionId           string
}

type OAuthInfoRequest struct {
	ReturnTo string // return to url to redirect after login
	Device   DeviceType
	Mode     ProviderLoginMode
	// SessionId 绑定模式下的发起会话
	SessionId string
}

type ProviderUserInfo struct {
	Issuer    string
	Subject   string
	Name      string
	AvatarURL string
	// Email 与 EmailVerified 是 provider 声明的邮箱及其验证状态；
	// 只有 EmailVerified=true 时才允许作为账号邮箱的来源。
	Email         string
	EmailVerified bool
	Raw           map[string]any
}

type Provider interface {
	Type() ProviderType
	// get basic login url
	OAuthInfo(ctx context.Context, request *OAuthInfoRequest) (*ProviderOAuthInfo, error)
	// GetUserInfo gets third-party user info out of code and verifier
	GetUserInfo(ctx context.Context, code string, state *OAuthState) (*ProviderUserInfo, error)
}

func CheckProviderType(s string) bool {
	_, ok := validProviderTypes[ProviderType(s)]
	return ok
}
