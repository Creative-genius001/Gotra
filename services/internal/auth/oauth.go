package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gotra/gotra/internal/config"
	errorMap "github.com/gotra/gotra/utils/error"
)

// OAuthProfile is the normalized identity returned by an external provider.
type OAuthProfile struct {
	ProviderUserID string
	Email          string
	Name           string
	AvatarURL      string
}

// providerEndpoints holds the OAuth2/OIDC URLs and scope for a provider.
type providerEndpoints struct {
	authURL     string
	tokenURL    string
	userinfoURL string
	scope       string
}

var endpoints = map[Provider]providerEndpoints{
	ProviderGoogle: {
		authURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		tokenURL: "https://oauth2.googleapis.com/token",
		scope:    "openid email profile",
	},
	ProviderGitHub: {
		authURL:  "https://github.com/login/oauth/authorize",
		tokenURL: "https://github.com/login/oauth/access_token",
		scope:    "read:user user:email",
	},
}

// ErrUnsupportedProvider is returned for an unknown OAuth provider.
var ErrUnsupportedProvider = errors.New("auth: unsupported oauth provider")

// OAuthManager performs the OAuth2/OIDC authorization-code exchange and profile
// fetch. OIDC endpoints are discovered from the issuer and cached.
type OAuthManager struct {
	cfg    *config.Config
	client *http.Client

	mu     sync.Mutex
	oidcEP *providerEndpoints
}

// NewOAuthManager constructs an OAuthManager.
func NewOAuthManager(cfg *config.Config) *OAuthManager {
	return &OAuthManager{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}}
}

func (m *OAuthManager) creds(provider Provider) (config.OAuthConfig, error) {
	switch provider {
	case ProviderGoogle:
		return m.cfg.Google, nil
	case ProviderGitHub:
		return m.cfg.GitHub, nil
	case ProviderOIDC:
		return config.OAuthConfig{
			ClientID:     m.cfg.OIDC.ClientID,
			ClientSecret: m.cfg.OIDC.ClientSecret,
			RedirectURL:  m.cfg.OIDC.RedirectURL,
		}, nil
	default:
		return config.OAuthConfig{}, errorMap.New(errorMap.CodeInvalidInput, "Oauth Service: Resolve Credentials", ErrUnsupportedProvider.Error())
	}
}

// resolveEndpoints returns a provider's endpoints, discovering OIDC on demand.
func (m *OAuthManager) resolveEndpoints(ctx context.Context, provider Provider) (providerEndpoints, error) {
	if ep, ok := endpoints[provider]; ok {
		return ep, nil
	}
	if provider == ProviderOIDC {
		return m.discoverOIDC(ctx)
	}
	return providerEndpoints{}, errorMap.New(errorMap.CodeInvalidInput, "Oauth Service: Resolve Endpoints", ErrUnsupportedProvider.Error())
}

func (m *OAuthManager) discoverOIDC(ctx context.Context) (providerEndpoints, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.oidcEP != nil {
		return *m.oidcEP, nil
	}
	var doc struct {
		AuthorizationEndpoint string `json:"authorization_endpoint"`
		TokenEndpoint         string `json:"token_endpoint"`
		UserinfoEndpoint      string `json:"userinfo_endpoint"`
	}
	url := strings.TrimRight(m.cfg.OIDC.Issuer, "/") + "/.well-known/openid-configuration"
	if err := m.getJSON(ctx, url, "", &doc); err != nil {
		return providerEndpoints{}, fmt.Errorf("oidc discovery: %w", err)
	}
	ep := providerEndpoints{
		authURL:     doc.AuthorizationEndpoint,
		tokenURL:    doc.TokenEndpoint,
		userinfoURL: doc.UserinfoEndpoint,
		scope:       "openid email profile",
	}
	m.oidcEP = &ep
	return ep, nil
}

// Configured reports whether the provider has credentials set.
func (m *OAuthManager) Configured(provider Provider) bool {
	if provider == ProviderOIDC {
		return m.cfg.OIDC.Configured()
	}
	c, err := m.creds(provider)
	return err == nil && c.ClientID != "" && c.ClientSecret != ""
}

// AuthCodeURL builds the provider authorization URL for the given CSRF state.
func (m *OAuthManager) AuthCodeURL(ctx context.Context, provider Provider, state string) (string, error) {
	c, err := m.creds(provider)
	if err != nil {
		return "", err
	}
	ep, err := m.resolveEndpoints(ctx, provider)
	if err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("client_id", c.ClientID)
	q.Set("redirect_uri", c.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", ep.scope)
	q.Set("state", state)
	if provider == ProviderGoogle {
		q.Set("access_type", "offline")
		q.Set("prompt", "select_account")
	}
	url := ep.authURL + "?" + q.Encode()
	return url, nil
}

// Exchange swaps an authorization code for an access token.
func (m *OAuthManager) Exchange(ctx context.Context, provider Provider, code string) (string, error) {
	c, err := m.creds(provider)
	if err != nil {
		return "", err
	}
	ep, err := m.resolveEndpoints(ctx, provider)
	if err != nil {
		return "", err
	}

	form := url.Values{}
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", c.RedirectURL)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errorMap.Wrap(err, errorMap.CodeInternal, "Oauth Service: Exchange", "error establishing connection")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return "", errorMap.Wrap(err, errorMap.CodeInternal, "Oauth Service", "error creating connection")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		errorMsg := fmt.Errorf("oauth token exchange failed (%d): %s", resp.StatusCode, string(body))
		return "", errorMap.New(errorMap.CodeInvalidInput, "Oauth Service: Exchange", errorMsg.Error())
	}

	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return "", errorMap.New(errorMap.CodeInvalidInput, "Oauth Service: Unmarshall body", "oauth token response missing access_token")
	}
	return tok.AccessToken, nil
}

// FetchProfile retrieves the normalized user profile using an access token.
func (m *OAuthManager) FetchProfile(ctx context.Context, provider Provider, accessToken string) (*OAuthProfile, error) {
	switch provider {
	case ProviderGoogle:
		return m.fetchGoogle(ctx, accessToken)
	case ProviderGitHub:
		return m.fetchGitHub(ctx, accessToken)
	case ProviderOIDC:
		ep, err := m.resolveEndpoints(ctx, provider)
		if err != nil {
			return nil, err
		}
		return m.fetchOIDC(ctx, ep.userinfoURL, accessToken)
	default:
		return nil, errorMap.New(errorMap.CodeInvalidInput, "Oauth Service: Fetch Profile", ErrUnsupportedProvider.Error())
	}
}

// fetchOIDC reads the standard OIDC userinfo claims.
func (m *OAuthManager) fetchOIDC(ctx context.Context, userinfoURL, token string) (*OAuthProfile, error) {
	var u struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := m.getJSON(ctx, userinfoURL, token, &u); err != nil {
		return nil, err
	}
	return &OAuthProfile{ProviderUserID: u.Sub, Email: u.Email, Name: u.Name, AvatarURL: u.Picture}, nil
}

func (m *OAuthManager) fetchGoogle(ctx context.Context, token string) (*OAuthProfile, error) {
	var u struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := m.getJSON(ctx, "https://www.googleapis.com/oauth2/v3/userinfo", token, &u); err != nil {
		return nil, err
	}
	return &OAuthProfile{ProviderUserID: u.Sub, Email: u.Email, Name: u.Name, AvatarURL: u.Picture}, nil
}

func (m *OAuthManager) fetchGitHub(ctx context.Context, token string) (*OAuthProfile, error) {
	var u struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}
	if err := m.getJSON(ctx, "https://api.github.com/user", token, &u); err != nil {
		return nil, err
	}

	email := u.Email
	if email == "" {
		// Primary email may be private; fetch verified primary explicitly.
		var emails []struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		if err := m.getJSON(ctx, "https://api.github.com/user/emails", token, &emails); err == nil {
			for _, e := range emails {
				if e.Primary && e.Verified {
					email = e.Email
					break
				}
			}
		}
	}

	name := u.Name
	if name == "" {
		name = u.Login
	}
	return &OAuthProfile{
		ProviderUserID: fmt.Sprintf("%d", u.ID),
		Email:          email,
		Name:           name,
		AvatarURL:      u.AvatarURL,
	}, nil
}

func (m *OAuthManager) getJSON(ctx context.Context, endpoint, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "gotra")

	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("oauth profile fetch failed (%d): %s", resp.StatusCode, string(body))
	}
	return json.Unmarshal(body, out)
}
