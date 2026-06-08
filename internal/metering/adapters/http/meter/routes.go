package meter

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/meters", h.Create)
	router.Get("/meters", h.List)
	router.Get("/meters/stats", h.ListStats)
	router.Get("/meters/{id}", h.Get)
	router.Put("/meters/{id}", h.Update)
	router.Delete("/meters/{id}", h.Delete)
}
