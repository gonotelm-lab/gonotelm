package google

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/idp"
)

const (
	// testNonce must match the `nonce` sent to GetUserInfo.
	testNonce = "test-nonce"
	// testKid is the key id of the test RSA key, both in the JWT header and JWKS.
	testKid = "test-kid"
	// testSubject is the stable account identifier returned by UserInfo.
	testSubject = "110169484474386276334"
	// testOIDCIssuer must match the `iss` claim of the minted id_token.
	testOIDCIssuer = "https://accounts.google.com"
	// testClientID is the client id every helper provider is built with.
	testClientID = "client-id"
)

func TestNew(t *testing.T) {
	t.Run("apply defaults", func(t *testing.T) {
		g, err := New(idp.Config{ClientID: testClientID, ClientSecret: "client-secret"})
		if err != nil {
			t.Fatalf("new failed: %v", err)
		}

		oauth2Config := g.GetOAuth2()
		if oauth2Config.Endpoint.AuthURL != defaultAuthorizationEndpoint {
			t.Fatalf("unexpected default auth url: %q", oauth2Config.Endpoint.AuthURL)
		}
		if oauth2Config.Endpoint.TokenURL != defaultTokenEndpoint {
			t.Fatalf("unexpected default token url: %q", oauth2Config.Endpoint.TokenURL)
		}
		if strings.Join(oauth2Config.Scopes, " ") != strings.Join(defaultScopes, " ") {
			t.Fatalf("unexpected default scopes: %v", oauth2Config.Scopes)
		}
		if g.discoveryURL != defaultDiscoveryURL {
			t.Fatalf("unexpected default discovery url: %q", g.discoveryURL)
		}
		if g.defaultUserInfoURL != defaultUserInfoEndpoint {
			t.Fatalf("unexpected default userinfo url: %q", g.defaultUserInfoURL)
		}
		if g.userInfoURL != "" {
			t.Fatalf("userinfo url must be resolved from discovery, got %q", g.userInfoURL)
		}
		if g.clientID != testClientID {
			t.Fatalf("unexpected client id: %q", g.clientID)
		}
		if g.oidcIssuer != defaultOIDCIssuer {
			t.Fatalf("unexpected default oidc issuer: %q", g.oidcIssuer)
		}
		if g.jwksURL != defaultJWKSURL {
			t.Fatalf("unexpected default jwks url: %q", g.jwksURL)
		}
		if g.httpClient == nil {
			t.Fatal("http client must not be nil")
		}
		if g.Type() != idp.TypeGoogle {
			t.Fatalf("unexpected provider type: %q", g.Type())
		}
	})

	t.Run("keep configured endpoints and scopes", func(t *testing.T) {
		g, err := New(idp.Config{
			ClientID: "client-id",
			AuthURL:  "https://example.com/auth",
			TokenURL: "https://example.com/token",
			Scopes:   []string{"openid"},
		},
			WithDiscoveryURL("https://example.com/discovery"),
			WithUserInfoURL("https://example.com/userinfo"),
		)
		if err != nil {
			t.Fatalf("new failed: %v", err)
		}

		oauth2Config := g.GetOAuth2()
		if oauth2Config.Endpoint.AuthURL != "https://example.com/auth" {
			t.Fatalf("unexpected auth url: %q", oauth2Config.Endpoint.AuthURL)
		}
		if oauth2Config.Endpoint.TokenURL != "https://example.com/token" {
			t.Fatalf("unexpected token url: %q", oauth2Config.Endpoint.TokenURL)
		}
		if len(oauth2Config.Scopes) != 1 || oauth2Config.Scopes[0] != "openid" {
			t.Fatalf("unexpected scopes: %v", oauth2Config.Scopes)
		}
		if g.discoveryURL != "https://example.com/discovery" {
			t.Fatalf("unexpected discovery url: %q", g.discoveryURL)
		}
		if g.userInfoURL != "https://example.com/userinfo" {
			t.Fatalf("unexpected userinfo url: %q", g.userInfoURL)
		}
	})
}

func TestAuthURL(t *testing.T) {
	g, err := New(idp.Config{
		ClientID:    "client-id",
		RedirectURL: "https://example.com/callback",
	})
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}

	authURL, state, err := g.AuthURL()
	if err != nil {
		t.Fatalf("auth url failed: %v", err)
	}

	query := authURL.Query()
	if got := query.Get("client_id"); got != "client-id" {
		t.Fatalf("unexpected client_id: %q", got)
	}
	if got := query.Get("redirect_uri"); got != "https://example.com/callback" {
		t.Fatalf("unexpected redirect_uri: %q", got)
	}
	if got := query.Get("response_type"); got != "code" {
		t.Fatalf("unexpected response_type: %q", got)
	}
	if got := query.Get("scope"); got != strings.Join(defaultScopes, " ") {
		t.Fatalf("unexpected scope: %q", got)
	}
	if got := query.Get("code_challenge_method"); got != "S256" {
		t.Fatalf("unexpected code_challenge_method: %q", got)
	}

	if state.State == "" || state.CodeVerifier == "" || state.CodeChallenge == "" {
		t.Fatalf("state is incomplete: %+v", state)
	}
	if got := query.Get("state"); got != state.State {
		t.Fatalf("state mismatch: query=%q state=%q", got, state.State)
	}
	if got := query.Get("code_challenge"); got != state.CodeChallenge {
		t.Fatalf("code challenge mismatch: query=%q state=%q", got, state.CodeChallenge)
	}

	// nonce is what ties the id_token back to this authorization request
	if state.Nonce == "" {
		t.Fatal("nonce must not be empty")
	}
	if got := query.Get("nonce"); got != state.Nonce {
		t.Fatalf("nonce mismatch: query=%q state=%q", got, state.Nonce)
	}
}

func TestGetUserInfo(t *testing.T) {
	key := newTestKey(t)
	idToken := signIDToken(t, key, testNonce, nil)

	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if r.Method != http.MethodPost {
				t.Errorf("unexpected token method: %s", r.Method)
				return
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse token form failed: %v", err)
				return
			}
			if got := r.Form.Get("grant_type"); got != "authorization_code" {
				t.Errorf("unexpected grant_type: %q", got)
				return
			}
			if got := r.Form.Get("code"); got != "auth-code" {
				t.Errorf("unexpected code: %q", got)
				return
			}
			if got := r.Form.Get("code_verifier"); got != "verifier" {
				t.Errorf("unexpected code_verifier: %q", got)
				return
			}

			writeToken(w, idToken)
		case "/jwks":
			writeJWKS(w, key)
		case "/userinfo":
			if got := r.Header.Get("Authorization"); got != "Bearer google-access-token" {
				t.Errorf("unexpected authorization header: %q", got)
				return
			}

			writeUserInfo(w)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))

	g := newProvider(t, srv.URL)
	userInfo, err := g.GetUserInfo(context.Background(), "auth-code", "verifier", testNonce)
	if err != nil {
		t.Fatalf("get user info failed: %v", err)
	}

	if userInfo.Issuer != issuer {
		t.Fatalf("unexpected issuer: %q", userInfo.Issuer)
	}
	if userInfo.Subject != testSubject {
		t.Fatalf("unexpected subject: %q", userInfo.Subject)
	}
	if userInfo.Name != "Ada Lovelace" {
		t.Fatalf("unexpected name: %q", userInfo.Name)
	}
	if userInfo.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("unexpected avatar url: %q", userInfo.AvatarURL)
	}
	if got := userInfo.Raw["email"]; got != "ada@example.com" {
		t.Fatalf("unexpected raw email: %v", got)
	}
	if got := userInfo.Raw["email_verified"]; got != true {
		t.Fatalf("unexpected raw email_verified: %v", got)
	}
	if got := userInfo.Raw["hd"]; got != "example.com" {
		t.Fatalf("unexpected raw hd: %v", got)
	}
}

func TestGetUserInfoDiscoversUserInfoEndpoint(t *testing.T) {
	var discoveryHits int

	key := newTestKey(t)
	idToken := signIDToken(t, key, testNonce, nil)

	var srv *httptest.Server
	srv = newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			writeToken(w, idToken)
		case "/jwks":
			writeJWKS(w, key)
		case "/discovery":
			discoveryHits++

			if got := r.Header.Get("Accept"); got != "application/json" {
				t.Errorf("unexpected accept header: %q", got)
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":            testOIDCIssuer,
				"userinfo_endpoint": srv.URL + "/discovered-userinfo",
			})
		case "/discovered-userinfo":
			writeUserInfo(w)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))

	g := newDiscoveryProvider(t, srv.URL)

	for range 2 {
		userInfo, err := g.GetUserInfo(context.Background(), "auth-code", "verifier", testNonce)
		if err != nil {
			t.Fatalf("get user info failed: %v", err)
		}
		if userInfo.Subject != testSubject {
			t.Fatalf("unexpected subject: %q", userInfo.Subject)
		}
	}

	// a successfully discovered endpoint is resolved only once
	if discoveryHits != 1 {
		t.Fatalf("expected 1 discovery request, got %d", discoveryHits)
	}
}

func TestGetUserInfoExplicitUserInfoURLSkipsDiscovery(t *testing.T) {
	key := newTestKey(t)
	idToken := signIDToken(t, key, testNonce, nil)

	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			writeToken(w, idToken)
		case "/jwks":
			writeJWKS(w, key)
		case "/userinfo":
			writeUserInfo(w)
		default:
			// /discovery must never be requested when the url is pinned
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))

	g := newProvider(t, srv.URL)
	if _, err := g.GetUserInfo(context.Background(), "auth-code", "verifier", testNonce); err != nil {
		t.Fatalf("get user info failed: %v", err)
	}
}

func TestGetUserInfoDiscoveryFailureFallsBack(t *testing.T) {
	var discoveryHits int

	key := newTestKey(t)
	idToken := signIDToken(t, key, testNonce, nil)

	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			writeToken(w, idToken)
		case "/jwks":
			writeJWKS(w, key)
		case "/discovery":
			discoveryHits++
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		case "/userinfo":
			writeUserInfo(w)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))

	g := newDiscoveryProvider(t, srv.URL)
	// the built-in default would hit the real Google endpoint
	g.defaultUserInfoURL = srv.URL + "/userinfo"

	for range 2 {
		if _, err := g.GetUserInfo(context.Background(), "auth-code", "verifier", testNonce); err != nil {
			t.Fatalf("get user info failed: %v", err)
		}
	}

	// a failed discovery is not cached, so the next login retries it
	if discoveryHits != 2 {
		t.Fatalf("expected 2 discovery requests, got %d", discoveryHits)
	}
}

// The id_token is the signed authentication assertion of the flow, every step of
// the OIDC Core §3.1.3.7 checklist must reject a bad one.
func TestGetUserInfoRejectsBadIDToken(t *testing.T) {
	key := newTestKey(t)
	otherKey := newTestKey(t)

	cases := []struct {
		name        string
		idToken     func(t *testing.T) string
		nonce       string
		userinfoSub string
		wantErr     string
	}{
		{
			name:        "nonce mismatch",
			idToken:     func(t *testing.T) string { return signIDToken(t, key, "another-nonce", nil) },
			nonce:       testNonce,
			userinfoSub: testSubject,
			wantErr:     "nonce",
		},
		{
			name:        "nonce missing in request",
			idToken:     func(t *testing.T) string { return signIDToken(t, key, testNonce, nil) },
			nonce:       "",
			userinfoSub: testSubject,
			wantErr:     "nonce",
		},
		{
			name:        "missing id_token",
			idToken:     func(t *testing.T) string { return "" },
			nonce:       testNonce,
			userinfoSub: testSubject,
			wantErr:     "id_token missing",
		},
		{
			name: "wrong audience",
			idToken: func(t *testing.T) string {
				return signIDToken(t, key, testNonce, func(c map[string]any) { c["aud"] = "someone-else" })
			},
			nonce:       testNonce,
			userinfoSub: testSubject,
			wantErr:     "audience",
		},
		{
			name: "expired",
			idToken: func(t *testing.T) string {
				return signIDToken(t, key, testNonce, func(c map[string]any) {
					c["exp"] = time.Now().Add(-time.Hour).Unix()
				})
			},
			nonce:       testNonce,
			userinfoSub: testSubject,
			wantErr:     "expired",
		},
		{
			name:        "forged signature",
			idToken:     func(t *testing.T) string { return signIDToken(t, otherKey, testNonce, nil) },
			nonce:       testNonce,
			userinfoSub: testSubject,
			wantErr:     "verify id_token",
		},
		{
			name:        "userinfo sub mismatch",
			idToken:     func(t *testing.T) string { return signIDToken(t, key, testNonce, nil) },
			nonce:       testNonce,
			userinfoSub: "another-sub",
			wantErr:     "does not match id_token sub",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/token":
					writeToken(w, tc.idToken(t))
				case "/jwks":
					writeJWKS(w, key)
				case "/userinfo":
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{
						"sub":  tc.userinfoSub,
						"name": "Ada Lovelace",
					})
				default:
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
			}))

			g := newProvider(t, srv.URL)
			_, err := g.GetUserInfo(context.Background(), "auth-code", "verifier", tc.nonce)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestGetUserInfoErrors(t *testing.T) {
	t.Run("token exchange failed", func(t *testing.T) {
		srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
		}))

		g := newProvider(t, srv.URL)
		_, err := g.GetUserInfo(context.Background(), "auth-code", "verifier", testNonce)
		if err == nil || !strings.Contains(err.Error(), "exchange oauth2 token") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("userinfo status not ok", func(t *testing.T) {
		key := newTestKey(t)
		idToken := signIDToken(t, key, testNonce, nil)

		srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/token":
				writeToken(w, idToken)
			case "/jwks":
				writeJWKS(w, key)
			default:
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			}
		}))

		g := newProvider(t, srv.URL)
		_, err := g.GetUserInfo(context.Background(), "auth-code", "verifier", testNonce)
		if err == nil || !strings.Contains(err.Error(), "unexpected status 401") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing subject", func(t *testing.T) {
		key := newTestKey(t)
		idToken := signIDToken(t, key, testNonce, nil)

		srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/token":
				writeToken(w, idToken)
			case "/jwks":
				writeJWKS(w, key)
			default:
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"name": "Ada Lovelace"})
			}
		}))

		g := newProvider(t, srv.URL)
		_, err := g.GetUserInfo(context.Background(), "auth-code", "verifier", testNonce)
		if err == nil || !strings.Contains(err.Error(), "missing sub") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("discovery document without userinfo endpoint", func(t *testing.T) {
		key := newTestKey(t)
		idToken := signIDToken(t, key, testNonce, nil)

		srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/token":
				writeToken(w, idToken)
			case "/jwks":
				writeJWKS(w, key)
			case "/discovery":
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"issuer": testOIDCIssuer})
			default:
				writeUserInfo(w)
			}
		}))

		g := newDiscoveryProvider(t, srv.URL)
		// the built-in default would hit the real Google endpoint
		g.defaultUserInfoURL = srv.URL + "/userinfo"

		if _, err := g.GetUserInfo(context.Background(), "auth-code", "verifier", testNonce); err != nil {
			t.Fatalf("get user info failed: %v", err)
		}
	})
}

func newTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return srv
}

// newProvider builds a Google provider pinned to the test server, the Discovery
// document is skipped.
func newProvider(t *testing.T, baseURL string) *Google {
	t.Helper()

	g, err := New(idp.Config{
		ClientID:     testClientID,
		ClientSecret: "client-secret",
		RedirectURL:  "https://example.com/callback",
		AuthURL:      baseURL + "/auth",
		TokenURL:     baseURL + "/token",
	}, WithUserInfoURL(baseURL+"/userinfo"))
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}

	return pinTestKeys(t, g, baseURL)
}

// newDiscoveryProvider builds a Google provider that resolves the UserInfo
// Endpoint from a Discovery document served by the test server.
func newDiscoveryProvider(t *testing.T, baseURL string) *Google {
	t.Helper()

	g, err := New(idp.Config{
		ClientID:     testClientID,
		ClientSecret: "client-secret",
		RedirectURL:  "https://example.com/callback",
		AuthURL:      baseURL + "/auth",
		TokenURL:     baseURL + "/token",
	}, WithDiscoveryURL(baseURL+"/discovery"))
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}

	return pinTestKeys(t, g, baseURL)
}

// pinTestKeys points the id_token verification at the test server instead of
// Google's real JWKS endpoint.
func pinTestKeys(t *testing.T, g *Google, baseURL string) *Google {
	t.Helper()

	g.jwksURL = baseURL + "/jwks"
	g.oidcIssuer = testOIDCIssuer

	return g
}

func newTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key failed: %v", err)
	}

	return key
}

// signIDToken mints an RS256 JWT with the claims Google would put in an
// id_token. The optional mutate hook tweaks a single claim.
func signIDToken(t *testing.T, key *rsa.PrivateKey, nonce string, mutate func(map[string]any)) string {
	t.Helper()

	claims := map[string]any{
		"iss":   testOIDCIssuer,
		"aud":   testClientID,
		"sub":   testSubject,
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
		"nonce": nonce,
	}
	if mutate != nil {
		mutate(claims)
	}

	header := map[string]any{"alg": "RS256", "typ": "JWT", "kid": testKid}

	hb, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header failed: %v", err)
	}
	cb, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims failed: %v", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(cb)

	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign id token failed: %v", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// writeJWKS serves the public half of the test key as a JWK Set.
func writeJWKS(w http.ResponseWriter, key *rsa.PrivateKey) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"keys": []map[string]any{{
			"kty": "RSA",
			"kid": testKid,
			"use": "sig",
			"alg": "RS256",
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}},
	})
}

func writeToken(w http.ResponseWriter, idToken string) {
	w.Header().Set("Content-Type", "application/json")

	body := map[string]any{
		"access_token": "google-access-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
	}
	if idToken != "" {
		body["id_token"] = idToken
	}

	_ = json.NewEncoder(w).Encode(body)
}

func writeUserInfo(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"sub":            testSubject,
		"name":           "Ada Lovelace",
		"given_name":     "Ada",
		"family_name":    "Lovelace",
		"picture":        "https://example.com/avatar.png",
		"email":          "ada@example.com",
		"email_verified": true,
		"hd":             "example.com",
	})
}
