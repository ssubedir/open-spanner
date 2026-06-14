package auth

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/auth/users", h.CreateUser)
	router.Get("/auth/session", h.GetSession)
	router.Delete("/auth/session", h.DeleteSession)
	router.Post("/auth/session/refresh", h.RefreshSession)
	router.Post("/auth/sessions", h.CreateSession)
}
