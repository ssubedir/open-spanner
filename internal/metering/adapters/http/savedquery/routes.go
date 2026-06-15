package savedquery

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Get("/usage/saved-queries", h.List)
	router.Post("/usage/saved-queries", h.Create)
	router.Put("/usage/saved-queries/{id}", h.Update)
	router.Delete("/usage/saved-queries/{id}", h.Delete)
}
