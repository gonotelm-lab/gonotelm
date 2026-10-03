package entity

import "context"

type ProviderType string

const (
	ProviderTypeGithub ProviderType = "github"
	ProviderTypeGoogle ProviderType = "google"
)

var validProviderTypes = map[ProviderType]bool{
	ProviderTypeGithub: true,
	ProviderTypeGoogle: true,
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

type ProviderLoginInfo struct {
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
}

func (info *ProviderLoginInfo) ToTransient() *TransientProviderLoginInfo {
	return &TransientProviderLoginInfo{
		State:               info.State,
		Nonce:               info.Nonce,
		CodeVerifier:        info.CodeVerifier,
		CodeChallenge:       info.CodeChallenge,
		CodeChallengeMethod: info.CodeChallengeMethod,
		ProviderType:        info.ProviderType,
		ReturnTo:            info.ReturnTo,
		Device:              info.Device,
	}
}

type TransientProviderLoginInfo struct {
	State               string
	Nonce               string
	CodeVerifier        string
	CodeChallenge       string
	CodeChallengeMethod string
	ReturnTo            string
	ProviderType        ProviderType
	Device              DeviceType
}

type LoginInfoRequest struct {
	ReturnTo string // return to url to redirect after login
	Device   DeviceType
}

type ProviderUserInfo struct {
	Issuer    string
	Subject   string
	Name      string
	AvatarURL string
	Raw       map[string]any
}

type Provider interface {
	Type() ProviderType
	// get basic login url
	LoginInfo(ctx context.Context, request *LoginInfoRequest) (*ProviderLoginInfo, error)
	// GetUserInfo gets third-party user info out of code and verifier
	GetUserInfo(ctx context.Context, code string, state *TransientProviderLoginInfo) (*ProviderUserInfo, error)
}

func CheckProviderType(s string) bool {
	_, ok := validProviderTypes[ProviderType(s)]
	return ok
}
