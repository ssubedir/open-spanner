package usage

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/usages", h.Create)
	router.Get("/usages", h.List)
}
