package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"

	"github.com/ssubedir/open-spanner/internal/config"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type OAuthIdentity struct {
	Provider      string
	Subject       string
	Email         string
	EmailVerified bool
}

type OAuthProvider interface {
	ID() string
	Name() string
	Enabled() bool
	RedirectURL() string
	Config(redirectURL string) *oauth2.Config
	Identity(ctx context.Context, token *oauth2.Token) (OAuthIdentity, error)
}

type idTokenVerifier interface {
	Validate(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error)
}

type googleIDTokenVerifier struct{}

func (googleIDTokenVerifier) Validate(ctx context.Context, rawIDToken string, audience string) (*idtoken.Payload, error) {
	validator, err := idtoken.NewValidator(ctx)
	if err != nil {
		return nil, err
	}
	return validator.Validate(ctx, rawIDToken, audience)
}

func defaultOAuthProviders(cfg config.OAuthConfigs, httpClient *http.Client, verifier idTokenVerifier) []OAuthProvider {
	return oauthProviders([]OAuthProvider{
		newGoogleOAuthProvider(cfg.Google, verifier),
		newGitHubOAuthProvider(cfg.GitHub, httpClient),
	})
}

func oauthProviders(providers []OAuthProvider) []OAuthProvider {
	result := make([]OAuthProvider, 0, len(providers))
	seen := map[string]struct{}{}
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		id := strings.ToLower(strings.TrimSpace(provider.ID()))
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, provider)
	}
	return result
}

type googleOAuthProvider struct {
	cfg      config.OAuthConfig
	verifier idTokenVerifier
}

func newGoogleOAuthProvider(cfg config.OAuthConfig, verifier idTokenVerifier) googleOAuthProvider {
	if verifier == nil {
		verifier = googleIDTokenVerifier{}
	}
	return googleOAuthProvider{cfg: cfg, verifier: verifier}
}

func (p googleOAuthProvider) ID() string {
	return "google"
}

func (p googleOAuthProvider) Name() string {
	return "Google"
}

func (p googleOAuthProvider) Enabled() bool {
	return p.cfg.Enabled && strings.TrimSpace(p.cfg.ClientID) != "" && strings.TrimSpace(p.cfg.ClientSecret) != ""
}

func (p googleOAuthProvider) RedirectURL() string {
	return p.cfg.RedirectURL
}

func (p googleOAuthProvider) Config(redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.cfg.ClientID,
		ClientSecret: p.cfg.ClientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
	}
}

func (p googleOAuthProvider) Identity(ctx context.Context, token *oauth2.Token) (OAuthIdentity, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(rawIDToken) == "" {
		return OAuthIdentity{}, domain.ErrUnauthorized
	}
	payload, err := p.verifier.Validate(ctx, rawIDToken, p.cfg.ClientID)
	if err != nil {
		return OAuthIdentity{}, err
	}
	email, _ := payload.Claims["email"].(string)
	return OAuthIdentity{
		Provider:      p.ID(),
		Subject:       payload.Subject,
		Email:         email,
		EmailVerified: claimBool(payload.Claims["email_verified"]),
	}, nil
}

type gitHubOAuthProvider struct {
	cfg        config.OAuthConfig
	httpClient *http.Client
}

func newGitHubOAuthProvider(cfg config.OAuthConfig, httpClient *http.Client) gitHubOAuthProvider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return gitHubOAuthProvider{cfg: cfg, httpClient: httpClient}
}

func (p gitHubOAuthProvider) ID() string {
	return "github"
}

func (p gitHubOAuthProvider) Name() string {
	return "GitHub"
}

func (p gitHubOAuthProvider) Enabled() bool {
	return p.cfg.Enabled && strings.TrimSpace(p.cfg.ClientID) != "" && strings.TrimSpace(p.cfg.ClientSecret) != ""
}

func (p gitHubOAuthProvider) RedirectURL() string {
	return p.cfg.RedirectURL
}

func (p gitHubOAuthProvider) Config(redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.cfg.ClientID,
		ClientSecret: p.cfg.ClientSecret,
		Endpoint:     github.Endpoint,
		RedirectURL:  redirectURL,
		Scopes:       []string{"read:user", "user:email"},
	}
}

func (p gitHubOAuthProvider) Identity(ctx context.Context, token *oauth2.Token) (OAuthIdentity, error) {
	var user struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
	}
	if err := p.getJSON(ctx, token, "https://api.github.com/user", &user); err != nil {
		return OAuthIdentity{}, err
	}
	if user.ID == 0 {
		return OAuthIdentity{}, domain.ErrUnauthorized
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := p.getJSON(ctx, token, "https://api.github.com/user/emails", &emails); err != nil {
		return OAuthIdentity{}, err
	}

	email := ""
	for _, candidate := range emails {
		if candidate.Primary && candidate.Verified && strings.TrimSpace(candidate.Email) != "" {
			email = candidate.Email
			break
		}
	}
	if email == "" {
		for _, candidate := range emails {
			if candidate.Verified && strings.TrimSpace(candidate.Email) != "" {
				email = candidate.Email
				break
			}
		}
	}
	if email == "" && strings.TrimSpace(user.Email) != "" {
		email = user.Email
	}
	if email == "" {
		return OAuthIdentity{}, domain.ErrUnauthorized
	}

	return OAuthIdentity{
		Provider:      p.ID(),
		Subject:       strconv.FormatInt(user.ID, 10),
		Email:         email,
		EmailVerified: true,
	}, nil
}

func (p gitHubOAuthProvider) getJSON(ctx context.Context, token *oauth2.Token, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("User-Agent", "open-spanner")

	res, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return domain.ErrUnauthorized
	}
	return json.NewDecoder(res.Body).Decode(target)
}
