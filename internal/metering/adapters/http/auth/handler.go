package auth

import (
	"net/http"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/request"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
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

	respond.JSON(w, http.StatusCreated, LoginResponse{
		Token:     session.Token,
		TokenType: session.TokenType,
		ExpiresAt: session.ExpiresAt.Format(time.RFC3339),
		User:      userResponse(session.User),
	})
}

func userResponse(user appauth.UserResult) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
	}
}
