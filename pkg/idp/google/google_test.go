package google

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/idp"
)

func TestNew(t *testing.T) {
	t.Run("apply defaults", func(t *testing.T) {
		g, err := New(idp.Config{ClientID: "client-id", ClientSecret: "client-secret"})
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
}

func TestGetUserInfo(t *testing.T) {
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

			writeToken(w)
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
	userInfo, err := g.GetUserInfo(context.Background(), "auth-code", "verifier")
	if err != nil {
		t.Fatalf("get user info failed: %v", err)
	}

	if userInfo.Issuer != issuer {
		t.Fatalf("unexpected issuer: %q", userInfo.Issuer)
	}
	if userInfo.Subject != "110169484474386276334" {
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

	var srv *httptest.Server
	srv = newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			writeToken(w)
		case "/discovery":
			discoveryHits++

			if got := r.Header.Get("Accept"); got != "application/json" {
				t.Errorf("unexpected accept header: %q", got)
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":            "https://accounts.google.com",
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
		userInfo, err := g.GetUserInfo(context.Background(), "auth-code", "verifier")
		if err != nil {
			t.Fatalf("get user info failed: %v", err)
		}
		if userInfo.Subject != "110169484474386276334" {
			t.Fatalf("unexpected subject: %q", userInfo.Subject)
		}
	}

	// a successfully discovered endpoint is resolved only once
	if discoveryHits != 1 {
		t.Fatalf("expected 1 discovery request, got %d", discoveryHits)
	}
}

func TestGetUserInfoExplicitUserInfoURLSkipsDiscovery(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			writeToken(w)
		case "/userinfo":
			writeUserInfo(w)
		default:
			// /discovery must never be requested when the url is pinned
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))

	g := newProvider(t, srv.URL)
	if _, err := g.GetUserInfo(context.Background(), "auth-code", "verifier"); err != nil {
		t.Fatalf("get user info failed: %v", err)
	}
}

func TestGetUserInfoDiscoveryFailureFallsBack(t *testing.T) {
	var discoveryHits int

	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			writeToken(w)
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
		if _, err := g.GetUserInfo(context.Background(), "auth-code", "verifier"); err != nil {
			t.Fatalf("get user info failed: %v", err)
		}
	}

	// a failed discovery is not cached, so the next login retries it
	if discoveryHits != 2 {
		t.Fatalf("expected 2 discovery requests, got %d", discoveryHits)
	}
}

func TestGetUserInfoErrors(t *testing.T) {
	t.Run("token exchange failed", func(t *testing.T) {
		srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
		}))

		g := newProvider(t, srv.URL)
		_, err := g.GetUserInfo(context.Background(), "auth-code", "verifier")
		if err == nil || !strings.Contains(err.Error(), "exchange oauth2 token") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("userinfo status not ok", func(t *testing.T) {
		srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/token" {
				writeToken(w)
				return
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}))

		g := newProvider(t, srv.URL)
		_, err := g.GetUserInfo(context.Background(), "auth-code", "verifier")
		if err == nil || !strings.Contains(err.Error(), "unexpected status 401") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing subject", func(t *testing.T) {
		srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/token" {
				writeToken(w)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "Ada Lovelace"})
		}))

		g := newProvider(t, srv.URL)
		_, err := g.GetUserInfo(context.Background(), "auth-code", "verifier")
		if err == nil || !strings.Contains(err.Error(), "missing sub") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("discovery document without userinfo endpoint", func(t *testing.T) {
		srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/token":
				writeToken(w)
			case "/discovery":
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"issuer": "https://accounts.google.com"})
			default:
				writeUserInfo(w)
			}
		}))

		g := newDiscoveryProvider(t, srv.URL)
		// the built-in default would hit the real Google endpoint
		g.defaultUserInfoURL = srv.URL + "/userinfo"

		if _, err := g.GetUserInfo(context.Background(), "auth-code", "verifier"); err != nil {
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
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://example.com/callback",
		AuthURL:      baseURL + "/auth",
		TokenURL:     baseURL + "/token",
	}, WithUserInfoURL(baseURL+"/userinfo"))
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}

	return g
}

// newDiscoveryProvider builds a Google provider that resolves the UserInfo
// Endpoint from a Discovery document served by the test server.
func newDiscoveryProvider(t *testing.T, baseURL string) *Google {
	t.Helper()

	g, err := New(idp.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://example.com/callback",
		AuthURL:      baseURL + "/auth",
		TokenURL:     baseURL + "/token",
	}, WithDiscoveryURL(baseURL+"/discovery"))
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}

	return g
}

func writeToken(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": "google-access-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
	})
}

func writeUserInfo(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"sub":            "110169484474386276334",
		"name":           "Ada Lovelace",
		"given_name":     "Ada",
		"family_name":    "Lovelace",
		"picture":        "https://example.com/avatar.png",
		"email":          "ada@example.com",
		"email_verified": true,
		"hd":             "example.com",
	})
}
