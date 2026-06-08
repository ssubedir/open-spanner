package meter

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/meters", h.Create)
	router.Get("/meters", h.List)
	router.Get("/meters/{id}", h.Get)
}
