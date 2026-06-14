package auth

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Get("/auth/api-keys", h.ListAPIKeys)
	router.Post("/auth/api-keys", h.CreateAPIKey)
	router.Delete("/auth/api-keys/{id}", h.DeleteAPIKey)
	router.Post("/auth/users", h.CreateUser)
	router.Get("/auth/session", h.GetSession)
	router.Delete("/auth/session", h.DeleteSession)
	router.Post("/auth/session/refresh", h.RefreshSession)
	router.Post("/auth/sessions", h.CreateSession)
}
