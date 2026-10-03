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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gonotelm-lab/gonotelm/pkg/idp"
	"golang.org/x/oauth2"
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

	// defaultOIDCIssuer is the `iss` claim Google puts in the id_token.
	defaultOIDCIssuer = "https://accounts.google.com"
	// defaultJWKSURL is the Discovery Document's `jwks_uri`, used to verify the
	// id_token signature.
	defaultJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"

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

	// clientID is the audience the id_token must have been issued for.
	clientID string
	// oidcIssuer is the expected `iss` claim of the id_token.
	oidcIssuer string
	// jwksURL is the JWKS endpoint backing the id_token signature.
	jwksURL string

	mu                  sync.Mutex
	resolvedUserInfoURL string
	verifier            *oidc.IDTokenVerifier
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
		clientID:           c.ClientID,
		oidcIssuer:         defaultOIDCIssuer,
		jwksURL:            defaultJWKSURL,
	}, nil
}

var _ idp.Provider = &Google{}

func (g *Google) Type() idp.Type {
	return idp.TypeGoogle
}

// GetUserInfo exchanges the authorization code for tokens, verifies the returned
// id_token, then reads the user profile from the UserInfo Endpoint.
//
// The token response already carries an `id_token` whose claims include the
// profile, but Google only guarantees `iss`/`sub`/`aud`/`exp`/`iat` there:
// `name` and `picture` are documented as "never guaranteed to be present".
// This endpoint is the documented way to obtain the profile:
// https://developers.google.com/identity/openid-connect/openid-connect#obtaininguserprofileinformation
func (g *Google) GetUserInfo(ctx context.Context, code, verifier, nonce string) (*idp.UserInfo, error) {
	token, err := g.ExchangeOAuth2Token(ctx, code, verifier)
	if err != nil {
		return nil, fmt.Errorf("exchange oauth2 token: %w", err)
	}

	idToken, err := g.verifyIDToken(ctx, token, nonce)
	if err != nil {
		return nil, err
	}

	user, err := g.getUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("get google user info: %w", err)
	}

	// UserInfo 的 sub 必须与 id_token 的 sub 完全一致，否则 UserInfo 的值不可用。
	// https://openid.net/specs/openid-connect-core-1_0.html#UserInfoResponse
	if user.Sub != idToken.Subject {
		return nil, fmt.Errorf("userinfo sub %q does not match id_token sub %q", user.Sub, idToken.Subject)
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

// verifyIDToken 校验 token 响应里的 id_token：签名(JWKS)、iss、aud、exp 以及 nonce。
// 这一步也能挡住"换回来的不是发给本 client 的 token"这类问题。
func (g *Google) verifyIDToken(ctx context.Context, token *oauth2.Token, nonce string) (*oidc.IDToken, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errors.New("id_token missing in token response, is the openid scope configured?")
	}

	idToken, err := g.verifierFor().Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify id_token: %w", err)
	}

	// go-oidc 只校验签名/iss/aud/exp，nonce 由调用方负责。
	if nonce == "" {
		return nil, errors.New("nonce is empty, cannot verify id_token")
	}
	if idToken.Nonce != nonce {
		return nil, errors.New("id_token nonce mismatch")
	}

	return idToken, nil
}

// verifierFor 惰性构建 id_token 校验器。这里用 RemoteKeySet 而不是 oidc.NewProvider，
// 因为 issuer 和 JWKS URL 是固定的，没必要为校验再多做一次 discovery。
func (g *Google) verifierFor() *oidc.IDTokenVerifier {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.verifier == nil {
		// NewRemoteKeySet 把 ctx 当配置载体(注入 http client)用，取消会被忽略，
		// 因此这里用 Background 并带上自己的 client。
		keySet := oidc.NewRemoteKeySet(
			oidc.ClientContext(context.Background(), g.httpClient),
			g.jwksURL,
		)
		g.verifier = oidc.NewVerifier(g.oidcIssuer, keySet, &oidc.Config{ClientID: g.clientID})
	}

	return g.verifier
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
