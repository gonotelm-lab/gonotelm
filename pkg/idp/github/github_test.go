package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/idp"
)

const (
	testClientID    = "client-id"
	testAccessToken = "github-access-token"
	testUserID      = 42
)

func TestGetUserInfoEmail(t *testing.T) {
	cases := []struct {
		name           string
		userEmail      string
		emails         []map[string]any
		wantEmail      string
		wantVerified   bool
		wantEmailsCall bool
	}{
		{
			name:         "public email from /user",
			userEmail:    "octocat@github.com",
			wantEmail:    "octocat@github.com",
			wantVerified: true,
		},
		{
			name: "primary verified email wins",
			emails: []map[string]any{
				{"email": "other@example.com", "primary": false, "verified": true, "visibility": "private"},
				{"email": "primary@example.com", "primary": true, "verified": true, "visibility": "private"},
			},
			wantEmail:      "primary@example.com",
			wantVerified:   true,
			wantEmailsCall: true,
		},
		{
			name: "fall back to any verified email",
			emails: []map[string]any{
				{"email": "unverified@example.com", "primary": true, "verified": false, "visibility": "private"},
				{"email": "verified@example.com", "primary": false, "verified": true, "visibility": "private"},
			},
			wantEmail:      "verified@example.com",
			wantVerified:   true,
			wantEmailsCall: true,
		},
		{
			name: "no verified email",
			emails: []map[string]any{
				{"email": "unverified@example.com", "primary": true, "verified": false, "visibility": "private"},
			},
			wantEmailsCall: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var emailsCalls int

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/token":
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{
						"access_token": testAccessToken,
						"token_type":   "bearer",
					})
				case "/user":
					assertAuthHeader(t, r)
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{
						"id":         testUserID,
						"login":      "octocat",
						"name":       "The Octocat",
						"email":      tc.userEmail,
						"avatar_url": "https://example.com/avatar.png",
					})
				case "/user/emails":
					emailsCalls++
					assertAuthHeader(t, r)
					if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
						t.Errorf("unexpected accept header: %q", got)
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(tc.emails)
				default:
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
			}))
			t.Cleanup(srv.Close)

			g, err := New(idp.Config{ClientID: testClientID, TokenURL: srv.URL + "/token"})
			if err != nil {
				t.Fatalf("new failed: %v", err)
			}
			g.apiBaseURL = srv.URL

			userInfo, err := g.GetUserInfo(context.Background(), "auth-code", "verifier", "")
			if err != nil {
				t.Fatalf("get user info failed: %v", err)
			}

			if userInfo.Email != tc.wantEmail {
				t.Fatalf("unexpected email: %q", userInfo.Email)
			}
			if userInfo.EmailVerified != tc.wantVerified {
				t.Fatalf("unexpected email verified: %v", userInfo.EmailVerified)
			}
			if tc.wantEmailsCall != (emailsCalls == 1) {
				t.Fatalf("expected emails call %v, got %d", tc.wantEmailsCall, emailsCalls)
			}

			// other UserInfo fields stay untouched
			if userInfo.Name != "octocat" {
				t.Fatalf("unexpected name: %q", userInfo.Name)
			}
			if userInfo.Subject != "42" {
				t.Fatalf("unexpected subject: %q", userInfo.Subject)
			}
		})
	}
}

func TestGetUserInfoEmailsStatusNotOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": testAccessToken,
				"token_type":   "bearer",
			})
		case "/user":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": testUserID, "login": "octocat"})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	g, err := New(idp.Config{ClientID: testClientID, TokenURL: srv.URL + "/token"})
	if err != nil {
		t.Fatalf("new failed: %v", err)
	}
	g.apiBaseURL = srv.URL

	_, err = g.GetUserInfo(context.Background(), "auth-code", "verifier", "")
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, "unexpected status 404") || !strings.Contains(got, "get github user emails") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertAuthHeader(t *testing.T, r *http.Request) {
	t.Helper()

	if got := r.Header.Get("Authorization"); got != "Bearer "+testAccessToken {
		t.Errorf("unexpected authorization header: %q", got)
	}
}
