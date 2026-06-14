package auth

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/auth/users", h.CreateUser)
	router.Post("/auth/sessions", h.CreateSession)
}
