// Package google implements "Sign in with Google" over OpenID Connect.
//
// Docs:
//   - OIDC guide:    https://developers.google.com/identity/openid-connect/openid-connect
//   - API reference: https://developers.google.com/identity/openid-connect/reference
//   - Discovery:     https://accounts.google.com/.well-known/openid-configuration
package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gonotelm-lab/gonotelm/pkg/idp"
)

// Google OpenID Connect endpoints, taken from the Discovery Document:
// https://accounts.google.com/.well-known/openid-configuration
const (
	// defaultDiscoveryURL is hard-coded on purpose, Google asks to pin it and to
	// read every other endpoint URL from the document it returns.
	defaultDiscoveryURL = "https://accounts.google.com/.well-known/openid-configuration"

	defaultAuthorizationEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenEndpoint         = "https://oauth2.googleapis.com/token"

	// defaultUserInfoEndpoint is only a fallback for when the Discovery document
	// cannot be reached, the value is normally read from `userinfo_endpoint`.
	defaultUserInfoEndpoint = "https://openidconnect.googleapis.com/v1/userinfo"

	// issuer identifies this provider in UserInfo. The account is identified by
	// the pair (issuer, subject), see idp.UserInfo.
	issuer = "google"
)

// defaultScopes is the minimal scope set for "Sign in with Google".
// Google requires `openid`, plus `email` and/or `profile`.
// https://developers.google.com/identity/openid-connect/reference#auth-endpoint
var defaultScopes = []string{"openid", "email", "profile"}

type Google struct {
	*idp.BaseProvider

	httpClient *http.Client

	// discoveryURL is the OpenID Connect Discovery document.
	discoveryURL string
	// userInfoURL is an explicit override, when set the Discovery document is not
	// consulted.
	userInfoURL string
	// defaultUserInfoURL is used when the Discovery document is unreachable.
	defaultUserInfoURL string

	mu                  sync.Mutex
	resolvedUserInfoURL string
}

func New(c idp.Config, opts ...Option) (*Google, error) {
	if c.AuthURL == "" {
		c.AuthURL = defaultAuthorizationEndpoint
	}
	if c.TokenURL == "" {
		c.TokenURL = defaultTokenEndpoint
	}
	if len(c.Scopes) == 0 {
		c.Scopes = defaultScopes
	}

	o := buildOptions(opts...)

	client := o.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	discoveryURL := o.discoveryURL
	if discoveryURL == "" {
		discoveryURL = defaultDiscoveryURL
	}

	return &Google{
		BaseProvider:       idp.NewBaseProvider(c),
		httpClient:         client,
		discoveryURL:       discoveryURL,
		userInfoURL:        o.userInfoURL,
		defaultUserInfoURL: defaultUserInfoEndpoint,
	}, nil
}

var _ idp.Provider = &Google{}

func (g *Google) Type() idp.Type {
	return idp.TypeGoogle
}

// GetUserInfo exchanges the authorization code for tokens, then reads the user
// profile from the OpenID Connect UserInfo Endpoint using the access token.
// The endpoint is bound to the access token that Google issued to this client,
// and returns the stable `sub` claim used as the account identifier.
//
// The token response already carries an `id_token` whose claims include the
// profile, but Google only guarantees `iss`/`sub`/`aud`/`exp`/`iat` there:
// `name` and `picture` are documented as "never guaranteed to be present".
// This endpoint is the documented way to obtain the profile:
// https://developers.google.com/identity/openid-connect/openid-connect#obtaininguserprofileinformation
func (g *Google) GetUserInfo(ctx context.Context, code, verifier string) (*idp.UserInfo, error) {
	token, err := g.ExchangeOAuth2Token(ctx, code, verifier)
	if err != nil {
		return nil, fmt.Errorf("exchange oauth2 token: %w", err)
	}

	user, err := g.getUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("get google user info: %w", err)
	}

	name := user.Name
	if name == "" {
		name = user.GivenName + " " + user.FamilyName
	}

	return &idp.UserInfo{
		Issuer:    issuer,
		Subject:   user.Sub,
		Name:      name,
		AvatarURL: user.Picture,
		Raw:       user.raw(),
	}, nil
}

// userInfo is the UserInfo Endpoint response body.
// https://developers.google.com/identity/openid-connect/reference#userinfo-endpoint
type userInfo struct {
	Sub           string `json:"sub"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Hd            string `json:"hd"`
}

func (u *userInfo) raw() map[string]any {
	return map[string]any{
		"sub":            u.Sub,
		"name":           u.Name,
		"given_name":     u.GivenName,
		"family_name":    u.FamilyName,
		"picture":        u.Picture,
		"email":          u.Email,
		"email_verified": u.EmailVerified,
		"hd":             u.Hd,
	}
}

func (g *Google) getUserInfo(ctx context.Context, accessToken string) (*userInfo, error) {
	userInfoURL := g.resolveUserInfoURL(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var u userInfo
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}

	// sub is the only required claim, it is the account identifier.
	if u.Sub == "" {
		return nil, fmt.Errorf("missing sub in userinfo response")
	}

	return &u, nil
}

// resolveUserInfoURL returns the UserInfo Endpoint, preferring an explicitly
// configured URL, then the `userinfo_endpoint` metadata value of the Discovery
// document. It falls back to defaultUserInfoURL when the document is
// unreachable, and only caches a successfully discovered value so a later login
// can retry the discovery.
func (g *Google) resolveUserInfoURL(ctx context.Context) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.resolvedUserInfoURL != "" {
		return g.resolvedUserInfoURL
	}

	if g.userInfoURL != "" {
		g.resolvedUserInfoURL = g.userInfoURL
		return g.resolvedUserInfoURL
	}

	userInfoURL, err := g.discoverUserInfoURL(ctx)
	if err != nil {
		slog.WarnContext(ctx, "discover google userinfo endpoint failed, falling back to default",
			slog.String("default_url", g.defaultUserInfoURL),
			slog.Any("err", err),
		)
		return g.defaultUserInfoURL
	}

	g.resolvedUserInfoURL = userInfoURL
	return g.resolvedUserInfoURL
}

// discoveryDocument holds the subset of the OpenID Connect Discovery document
// this client needs.
// https://developers.google.com/identity/openid-connect/openid-connect#discovery-document
type discoveryDocument struct {
	Issuer           string `json:"issuer"`
	UserInfoEndpoint string `json:"userinfo_endpoint"`
}

func (g *Google) discoverUserInfoURL(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.discoveryURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var doc discoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", err
	}
	if doc.UserInfoEndpoint == "" {
		return "", fmt.Errorf("missing userinfo_endpoint in discovery document")
	}

	return doc.UserInfoEndpoint, nil
}
