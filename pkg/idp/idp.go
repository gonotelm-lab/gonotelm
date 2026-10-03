package idp

import (
	"context"
	"fmt"
	"net/url"

	"golang.org/x/oauth2"
)

type Type string

const (
	TypeGithub Type = "github"
	TypeGoogle Type = "google"
)

type UserInfo struct {
	Issuer    string
	Subject   string
	Name      string
	AvatarURL string
	Raw       map[string]any
}

type Provider interface {
	Type() Type
	AuthURL() (*url.URL, *State, error)
	// GetUserInfo gets the user profile out of code and verifier. nonce is the
	// value sent in AuthURL, OIDC providers must check it against the id_token.
	GetUserInfo(ctx context.Context, code, verifier, nonce string) (*UserInfo, error)
}

type BaseProvider struct {
	c    Config
	oa2c oauth2.Config
}

func NewBaseProvider(c Config) *BaseProvider {
	return &BaseProvider{
		c: c,
		oa2c: oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			RedirectURL:  c.RedirectURL,
			Scopes:       c.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  c.AuthURL,
				TokenURL: c.TokenURL,
			},
		},
	}
}

func (b *BaseProvider) GetOAuth2() *oauth2.Config {
	return &b.oa2c
}

func (b *BaseProvider) GetState() (State, error) {
	newState := State{}

	state, err := RandomString(32)
	if err != nil {
		return newState, err
	}

	nonce, err := RandomString(32)
	if err != nil {
		return newState, err
	}

	codeVerifier, err := RandomString(32)
	if err != nil {
		return newState, err
	}

	codeChallenge := oauth2.S256ChallengeFromVerifier(codeVerifier)

	newState.State = state
	newState.Nonce = nonce
	newState.CodeVerifier = codeVerifier
	newState.CodeChallenge = codeChallenge
	newState.CodeChallengeMethod = CodeChallengeMethodS256

	return newState, nil
}

func (b *BaseProvider) AuthURL() (*url.URL, *State, error) {
	state, err := b.GetState()
	if err != nil {
		return nil, nil, err
	}

	u := b.oa2c.AuthCodeURL(
		state.State,
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(state.CodeVerifier),
		// OIDC provider 会把 nonce 写进 id_token；非 OIDC 的 provider(GitHub)
		// 没有 id_token，会忽略这个参数。
		oauth2.SetAuthURLParam("nonce", state.Nonce),
	)
	res, err := url.Parse(u)
	if err != nil {
		return nil, nil, err
	}

	return res, &state, nil
}

func (b *BaseProvider) ExchangeOAuth2Token(ctx context.Context, code, verifier string) (*oauth2.Token, error) {
	oauth2Token, err := b.oa2c.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchange code for token err: %w", err)
	}

	return oauth2Token, nil
}
