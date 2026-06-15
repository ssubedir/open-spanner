package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/request"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

const (
	accessCookieName  = "open_spanner_access"
	refreshCookieName = "open_spanner_refresh"
)

type Handler struct {
	service appauth.Service
}

func NewHandler(service appauth.Service) *Handler {
	return &Handler{service: service}
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

	key, err := h.service.CreateAPIKey(r.Context(), appauth.CreateAPIKeyCommand{
		UserID: user.ID,
		Name:   req.Name,
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
	token, err := tokenFromCookie(r, accessCookieName)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	user, err := h.service.AuthenticateSession(r.Context(), token)
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
	token, err := tokenFromCookie(r, accessCookieName)
	if err != nil {
		return appauth.UserResult{}, err
	}
	return h.service.AuthenticateSession(r.Context(), token)
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
	return APIKeyResponse{
		ID:         key.ID,
		Name:       key.Name,
		Prefix:     key.Prefix,
		CreatedAt:  key.CreatedAt.Format(time.RFC3339),
		LastUsedAt: lastUsedAt,
	}
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

func (h *Handler) AuthenticateRequest(r *http.Request) error {
	_, err := h.authenticateRequestUser(r)
	return err
}

func (h *Handler) authenticateRequestUser(r *http.Request) (appauth.UserResult, error) {
	if token := apiKeyFromRequest(r); token != "" {
		return h.service.AuthenticateAPIKey(r.Context(), token)
	}

	token, err := tokenFromCookie(r, accessCookieName)
	if err != nil {
		return appauth.UserResult{}, err
	}
	return h.service.AuthenticateSession(r.Context(), token)
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
