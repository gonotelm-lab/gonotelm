package entity

import "context"

type ProviderType string

const (
	ProviderTypeGithub ProviderType = "github"
)

var validProviderTypes = map[ProviderType]bool{
	ProviderTypeGithub: true,
}

func (t ProviderType) String() string {
	return string(t)
}

func (t ProviderType) Iss() string {
	switch t {
	case ProviderTypeGithub:
		return "https://github.com/login/oauth"
	default:
		return ""
	}
}

type ProviderLoginInfo struct {
	URL                 string       // full login url with query params to redirect to with http status code 302
	ClientId            string       // client_id
	State               string       // random state
	CodeVerifier        string       // random code verifier
	CodeChallenge       string       // usually base64url(sha256(codeVerifier))
	CodeChallengeMethod string       // code challenge method, always "S256"
	ProviderType        ProviderType // provider type
	ReturnTo            string       // return to url to redirect after login
}

func (info *ProviderLoginInfo) ToTransient() *TransientProviderLoginInfo {
	return &TransientProviderLoginInfo{
		State:               info.State,
		CodeVerifier:        info.CodeVerifier,
		CodeChallenge:       info.CodeChallenge,
		CodeChallengeMethod: info.CodeChallengeMethod,
		ProviderType:        info.ProviderType,
		ReturnTo:            info.ReturnTo,
	}
}

type TransientProviderLoginInfo struct {
	State               string
	CodeVerifier        string
	CodeChallenge       string
	CodeChallengeMethod string
	ReturnTo            string
	ProviderType        ProviderType
}

type LoginInfoRequest struct {
	ReturnTo string // return to url to redirect after login
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
