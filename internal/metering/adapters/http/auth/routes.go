package auth

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	h.RegisterPublicRoutes(router)
	router.Group(func(session chi.Router) {
		session.Use(h.RequireSession)
		h.RegisterSessionRoutes(session)
	})
}

func (h *Handler) RegisterPublicRoutes(router chi.Router) {
	router.Get("/auth/oauth/{provider}", h.StartOAuth)
	router.Get("/auth/oauth/{provider}/callback", h.CompleteOAuth)
	router.Get("/auth/providers", h.ListOAuthProviders)
	router.Post("/auth/users", h.CreateUser)
	router.Delete("/auth/session", h.DeleteSession)
	router.Post("/auth/session/refresh", h.RefreshSession)
	router.Post("/auth/sessions", h.CreateSession)
}

func (h *Handler) RegisterSessionRoutes(router chi.Router) {
	router.Get("/auth/api-keys", h.ListAPIKeys)
	router.Post("/auth/api-keys", h.CreateAPIKey)
	router.Delete("/auth/api-keys/{id}", h.DeleteAPIKey)
	router.Get("/auth/session", h.GetSession)
}
