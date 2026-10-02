package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gonotelm-lab/gonotelm/pkg/idp"
)

// OAuth flow endpoints, see:
// Authorize: https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps
// Exchange:  https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#2-users-are-redirected-back-to-your-site-with-a-code
const (
	defaultAuthorizationEndpoint = "https://github.com/login/oauth/authorize"
	defaultTokenEndpoint         = "https://github.com/login/oauth/access_token"
	defaultAPIBaseURL            = "https://api.github.com"
	issuer                       = "github"
)

type GitHub struct {
	*idp.BaseProvider

	httpClient *http.Client
	apiBaseURL string
}

func New(c idp.Config, opts ...Option) (*GitHub, error) {
	if c.AuthURL == "" {
		c.AuthURL = defaultAuthorizationEndpoint
	}
	if c.TokenURL == "" {
		c.TokenURL = defaultTokenEndpoint
	}

	o := buildOptions(opts...)
	client := o.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	return &GitHub{
		BaseProvider: idp.NewBaseProvider(c),
		httpClient:   client,
		apiBaseURL:   defaultAPIBaseURL,
	}, nil
}

var _ idp.Provider = &GitHub{}

func (g *GitHub) Type() idp.Type {
	return idp.TypeGithub
}

func (g *GitHub) GetUserInfo(ctx context.Context, code, verifier string) (*idp.UserInfo, error) {
	token, err := g.ExchangeOAuth2Token(ctx, code, verifier)
	if err != nil {
		return nil, fmt.Errorf("exchange oauth2 token: %w", err)
	}

	user, err := g.getUser(ctx, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("get github user: %w", err)
	}

	return &idp.UserInfo{
		Issuer:  issuer,
		Subject: strconv.FormatInt(user.ID, 10),
		Name:    user.Login,
		Raw:     user.raw(),
	}, nil
}

type user struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

func (u *user) raw() map[string]any {
	return map[string]any{
		"id":         u.ID,
		"login":      u.Login,
		"name":       u.Name,
		"email":      u.Email,
		"avatar_url": u.AvatarURL,
	}
}

// getUser calls the GitHub "Get the authenticated user" API.
// Docs: https://docs.github.com/en/rest/users/users#get-the-authenticated-user
func (g *GitHub) getUser(ctx context.Context, accessToken string) (*user, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.apiBaseURL+"/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var u user
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}

	return &u, nil
}
