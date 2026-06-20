package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/config"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/request"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

const (
	accessCookieName        = "open_spanner_access"
	oauthRedirectCookieName = "open_spanner_oauth_redirect"
	oauthStateCookieName    = "open_spanner_oauth_state"
	refreshCookieName       = "open_spanner_refresh"
)

type Handler struct {
	httpClient       *http.Client
	oauthFailurePath string
	oauthProviders   []OAuthProvider
	oauthSuccessPath string
	service          appauth.Service
	verifier         idTokenVerifier
}

type HandlerOptions struct {
	HTTPClient       *http.Client
	OAuth            config.OAuthConfigs
	OAuthFailurePath string
	OAuthSuccessPath string
	OAuthProviders   []OAuthProvider
	Verifier         idTokenVerifier
}

func NewHandler(service appauth.Service, options ...HandlerOptions) *Handler {
	handler := &Handler{
		httpClient:       http.DefaultClient,
		oauthFailurePath: "/login",
		oauthSuccessPath: "/overview",
		service:          service,
		verifier:         googleIDTokenVerifier{},
	}
	if len(options) > 0 {
		if options[0].HTTPClient != nil {
			handler.httpClient = options[0].HTTPClient
		}
		if strings.TrimSpace(options[0].OAuthFailurePath) != "" {
			handler.oauthFailurePath = options[0].OAuthFailurePath
		}
		if strings.TrimSpace(options[0].OAuthSuccessPath) != "" {
			handler.oauthSuccessPath = options[0].OAuthSuccessPath
		}
		if options[0].Verifier != nil {
			handler.verifier = options[0].Verifier
		}
		handler.oauthProviders = oauthProviders(options[0].OAuthProviders)
		if len(handler.oauthProviders) == 0 {
			handler.oauthProviders = defaultOAuthProviders(
				options[0].OAuth,
				handler.httpClient,
				handler.verifier,
			)
		}
	}
	if len(handler.oauthProviders) == 0 {
		handler.oauthProviders = defaultOAuthProviders(config.OAuthConfigs{}, handler.httpClient, handler.verifier)
	}
	return handler
}

// CreateUser creates an auth user.
//
// @Summary Create auth user
// @ID createAuthUser
// @Tags auth
// @Accept json
// @Produce json
// @Param request body CreateUserRequest true "User"
// @Success 201 {object} UserResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/auth/users [post]
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}

	user, err := h.service.CreateUser(r.Context(), appauth.CreateUserCommand{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, userResponse(user))
}

// CreateSession logs in an auth user.
//
// @Summary Create auth session
// @ID createAuthSession
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 201 {object} LoginResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 401 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/auth/sessions [post]
func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}

	session, err := h.service.Login(r.Context(), appauth.LoginCommand{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	setAuthCookie(w, r, accessCookieName, session.AccessToken, session.AccessExpiresAt)
	setAuthCookie(w, r, refreshCookieName, session.RefreshToken, session.RefreshExpiresAt)
	respond.JSON(w, http.StatusCreated, LoginResponse{
		ExpiresAt: session.AccessExpiresAt.Format(time.RFC3339),
		User:      userResponse(session.User),
	})
}

// ListOAuthProviders lists enabled social login providers.
//
// @Summary List OAuth providers
// @ID listOAuthProviders
// @Tags auth
// @Produce json
// @Success 200 {object} OAuthProviderListResponse
// @Router /v1/auth/providers [get]
func (h *Handler) ListOAuthProviders(w http.ResponseWriter, r *http.Request) {
	providers := make([]OAuthProviderResponse, 0, len(h.oauthProviders))
	for _, provider := range h.oauthProviders {
		providers = append(providers, OAuthProviderResponse{
			Enabled: provider.Enabled(),
			ID:      provider.ID(),
			Name:    provider.Name(),
		})
	}
	respond.JSON(w, http.StatusOK, OAuthProviderListResponse{Items: providers})
}

// StartOAuth redirects the user to an OAuth/OIDC provider.
//
// @Summary Start OAuth login
// @ID startOAuth
// @Tags auth
// @Param provider path string true "OAuth provider"
// @Success 302
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/auth/oauth/{provider} [get]
func (h *Handler) StartOAuth(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.oauthProvider(oauthProviderID(r))
	if !ok {
		respond.Error(w, http.StatusNotFound, "not_found", "oauth provider is not enabled")
		return
	}

	state, err := randomURLToken(32)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	redirectURL := h.oauthRedirectURL(r, provider)
	oauthConfig := provider.Config(redirectURL)
	if oauthConfig == nil {
		respond.Error(w, http.StatusNotFound, "not_found", "oauth provider is not enabled")
		return
	}
	setOAuthStateCookie(w, r, state)
	setOAuthRedirectCookie(w, r, redirectURL)
	http.Redirect(w, r, oauthConfig.AuthCodeURL(state), http.StatusFound)
}

// CompleteOAuth handles an OAuth/OIDC provider callback.
//
// @Summary Complete OAuth login
// @ID completeOAuth
// @Tags auth
// @Param provider path string true "OAuth provider"
// @Success 302
// @Failure 302
// @Router /v1/auth/oauth/{provider}/callback [get]
func (h *Handler) CompleteOAuth(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.oauthProvider(oauthProviderID(r))
	if !ok {
		http.Redirect(w, r, h.oauthFailurePath, http.StatusFound)
		return
	}
	defer func() {
		clearOAuthStateCookie(w, r)
		clearOAuthRedirectCookie(w, r)
	}()

	state, err := tokenFromCookie(r, oauthStateCookieName)
	if err != nil || subtle.ConstantTimeCompare([]byte(state), []byte(r.URL.Query().Get("state"))) != 1 {
		http.Redirect(w, r, h.oauthFailurePath, http.StatusFound)
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		http.Redirect(w, r, h.oauthFailurePath, http.StatusFound)
		return
	}

	redirectURL := h.oauthRedirectURL(r, provider)
	if cookieRedirectURL, err := tokenFromCookie(r, oauthRedirectCookieName); err == nil {
		redirectURL = cookieRedirectURL
	}
	oauthConfig := provider.Config(redirectURL)
	if oauthConfig == nil {
		http.Redirect(w, r, h.oauthFailurePath, http.StatusFound)
		return
	}
	token, err := oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		http.Redirect(w, r, h.oauthFailurePath, http.StatusFound)
		return
	}
	identity, err := provider.Identity(r.Context(), token)
	if err != nil {
		http.Redirect(w, r, h.oauthFailurePath, http.StatusFound)
		return
	}

	session, err := h.service.LoginWithExternalIdentity(r.Context(), appauth.ExternalIdentityLoginCommand{
		Provider:      identity.Provider,
		Subject:       identity.Subject,
		Email:         identity.Email,
		EmailVerified: identity.EmailVerified,
	})
	if err != nil {
		http.Redirect(w, r, h.oauthFailurePath, http.StatusFound)
		return
	}

	setAuthCookie(w, r, accessCookieName, session.AccessToken, session.AccessExpiresAt)
	setAuthCookie(w, r, refreshCookieName, session.RefreshToken, session.RefreshExpiresAt)
	http.Redirect(w, r, h.oauthSuccessPath, http.StatusFound)
}

// ListAPIKeys lists API keys for the current user.
//
// @Summary List API keys
// @ID listAPIKeys
// @Tags auth
// @Produce json
// @Success 200 {object} APIKeyListResponse
// @Failure 401 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/auth/api-keys [get]
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	user, err := h.currentUser(r)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	keys, err := h.service.ListAPIKeys(r.Context(), user.ID)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	res := make([]APIKeyResponse, 0, len(keys))
	for _, key := range keys {
		res = append(res, apiKeyResponse(key))
	}
	respond.JSON(w, http.StatusOK, APIKeyListResponse{Items: res})
}

// CreateAPIKey creates an API key for SDK access.
//
// @Summary Create API key
// @ID createAPIKey
// @Tags auth
// @Accept json
// @Produce json
// @Param request body CreateAPIKeyRequest true "API key"
// @Success 201 {object} APIKeyCreateResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 401 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/auth/api-keys [post]
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user, err := h.currentUser(r)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	var req CreateAPIKeyRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}
	expiresAt, err := request.OptionalTime("expires_at", req.ExpiresAt)
	if err != nil {
		respond.ValidationError(w, err)
		return
	}
	var expiresAtPtr *time.Time
	if !expiresAt.IsZero() {
		expiresAtPtr = &expiresAt
	}

	key, err := h.service.CreateAPIKey(r.Context(), appauth.CreateAPIKeyCommand{
		UserID:        user.ID,
		Name:          req.Name,
		Scopes:        req.Scopes,
		AllowedMeters: req.AllowedMeters,
		ExpiresAt:     expiresAtPtr,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, APIKeyCreateResponse{
		APIKeyResponse: apiKeyResponse(key.APIKeyResult),
		Key:            key.Key,
	})
}

// DeleteAPIKey deletes an API key for the current user.
//
// @Summary Delete API key
// @ID deleteAPIKey
// @Tags auth
// @Produce json
// @Param id path string true "API key ID"
// @Success 204
// @Failure 401 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/auth/api-keys/{id} [delete]
func (h *Handler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	user, err := h.currentUser(r)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	if err := h.service.DeleteAPIKey(r.Context(), user.ID, chi.URLParam(r, "id")); err != nil {
		respond.ServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RefreshSession rotates auth cookies with the current refresh token.
//
// @Summary Refresh auth session
// @ID refreshAuthSession
// @Tags auth
// @Produce json
// @Success 200 {object} RefreshResponse
// @Failure 401 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/auth/session/refresh [post]
func (h *Handler) RefreshSession(w http.ResponseWriter, r *http.Request) {
	token, err := tokenFromCookie(r, refreshCookieName)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	session, err := h.service.RefreshSession(r.Context(), token)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	setAuthCookie(w, r, accessCookieName, session.AccessToken, session.AccessExpiresAt)
	setAuthCookie(w, r, refreshCookieName, session.RefreshToken, session.RefreshExpiresAt)
	respond.JSON(w, http.StatusOK, RefreshResponse{
		ExpiresAt: session.AccessExpiresAt.Format(time.RFC3339),
		User:      userResponse(session.User),
	})
}

// GetSession gets the current auth session.
//
// @Summary Get auth session
// @ID getAuthSession
// @Tags auth
// @Produce json
// @Success 200 {object} SessionResponse
// @Failure 401 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/auth/session [get]
func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	user, err := h.currentUser(r)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, SessionResponse{User: userResponse(user)})
}

// DeleteSession logs out the current auth session.
//
// @Summary Delete auth session
// @ID deleteAuthSession
// @Tags auth
// @Produce json
// @Success 204
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/auth/session [delete]
func (h *Handler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	if token, err := tokenFromCookie(r, accessCookieName); err == nil {
		if err := h.service.DeleteSession(r.Context(), token); err != nil {
			respond.ServiceError(w, err)
			return
		}
	}
	if token, err := tokenFromCookie(r, refreshCookieName); err == nil {
		if err := h.service.DeleteSession(r.Context(), token); err != nil {
			respond.ServiceError(w, err)
			return
		}
	}

	clearAuthCookie(w, r, accessCookieName)
	clearAuthCookie(w, r, refreshCookieName)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) currentUser(r *http.Request) (appauth.UserResult, error) {
	if user, ok := UserFromContext(r.Context()); ok {
		return user, nil
	}
	principal, err := h.currentPrincipal(r)
	if err != nil {
		return appauth.UserResult{}, err
	}
	return principal.User, nil
}

func (h *Handler) currentPrincipal(r *http.Request) (appauth.Principal, error) {
	token, err := tokenFromCookie(r, accessCookieName)
	if err != nil {
		return appauth.Principal{}, err
	}
	return h.service.AuthenticateSessionPrincipal(r.Context(), token)
}

func userResponse(user appauth.UserResult) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
	}
}

func apiKeyResponse(key appauth.APIKeyResult) APIKeyResponse {
	var lastUsedAt *string
	if key.LastUsedAt != nil {
		formatted := key.LastUsedAt.Format(time.RFC3339)
		lastUsedAt = &formatted
	}
	var expiresAt *string
	if key.ExpiresAt != nil {
		formatted := key.ExpiresAt.Format(time.RFC3339)
		expiresAt = &formatted
	}
	var revokedAt *string
	if key.RevokedAt != nil {
		formatted := key.RevokedAt.Format(time.RFC3339)
		revokedAt = &formatted
	}
	return APIKeyResponse{
		ID:            key.ID,
		Name:          key.Name,
		Prefix:        key.Prefix,
		Scopes:        stringSlice(key.Scopes),
		AllowedMeters: stringSlice(key.AllowedMeters),
		ExpiresAt:     expiresAt,
		RevokedAt:     revokedAt,
		CreatedAt:     key.CreatedAt.Format(time.RFC3339),
		LastUsedAt:    lastUsedAt,
	}
}

func stringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func tokenFromCookie(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return "", domain.ErrUnauthorized
		}
		return "", err
	}
	if cookie.Value == "" {
		return "", domain.ErrUnauthorized
	}
	return cookie.Value, nil
}

func (h *Handler) authenticateRequestPrincipal(r *http.Request) (appauth.Principal, error) {
	if token := apiKeyFromRequest(r); token != "" {
		return h.service.AuthenticateAPIKeyPrincipal(r.Context(), token)
	}

	return h.currentPrincipal(r)
}

func apiKeyFromRequest(r *http.Request) string {
	if key := strings.TrimSpace(r.Header.Get("X-Open-Spanner-API-Key")); key != "" {
		return key
	}
	if key := strings.TrimSpace(r.Header.Get("X-API-Key")); key != "" {
		return key
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return ""
	}
	if token, ok := strings.CutPrefix(auth, "Bearer "); ok {
		return strings.TrimSpace(token)
	}
	if token, ok := strings.CutPrefix(auth, "bearer "); ok {
		return strings.TrimSpace(token)
	}
	return ""
}

func oauthProviderID(r *http.Request) string {
	return strings.ToLower(strings.TrimSpace(chi.URLParam(r, "provider")))
}

func (h *Handler) oauthRedirectURL(r *http.Request, provider OAuthProvider) string {
	if origin := redirectOrigin(r.URL.Query().Get("redirect_origin")); origin != "" {
		return origin + "/v1/auth/oauth/" + provider.ID() + "/callback"
	}
	redirectURL := strings.TrimSpace(provider.RedirectURL())
	if redirectURL == "" {
		redirectURL = requestURL(r, "/v1/auth/oauth/"+provider.ID()+"/callback")
	}
	return redirectURL
}

func redirectOrigin(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func requestURL(r *http.Request, path string) string {
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, path)
}

func (h *Handler) oauthProvider(id string) (OAuthProvider, bool) {
	for _, provider := range h.oauthProviders {
		if strings.EqualFold(provider.ID(), id) && provider.Enabled() {
			return provider, true
		}
	}
	return nil, false
}

func claimBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true")
	default:
		return false
	}
}

func randomURLToken(byteCount int) (string, error) {
	data := make([]byte, byteCount)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func setAuthCookie(w http.ResponseWriter, r *http.Request, name string, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}

func setOAuthStateCookie(w http.ResponseWriter, r *http.Request, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/v1/auth/oauth",
		MaxAge:   int((10 * time.Minute).Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}

func setOAuthRedirectCookie(w http.ResponseWriter, r *http.Request, redirectURL string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthRedirectCookieName,
		Value:    redirectURL,
		Path:     "/v1/auth/oauth",
		MaxAge:   int((10 * time.Minute).Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearOAuthStateCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/v1/auth/oauth",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearOAuthRedirectCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthRedirectCookieName,
		Value:    "",
		Path:     "/v1/auth/oauth",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearAuthCookie(w http.ResponseWriter, r *http.Request, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}
