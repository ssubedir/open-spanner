package alert

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/alerts", h.Create)
	router.Get("/alerts", h.List)
	router.Post("/alerts/destinations", h.CreateDestination)
	router.Get("/alerts/destinations", h.ListDestinations)
	router.Put("/alerts/destinations/{id}", h.UpdateDestination)
	router.Delete("/alerts/destinations/{id}", h.DeleteDestination)
	router.Post("/alerts/destinations/{id}/webhook-secret/rotate", h.RotateDestinationSecret)
	router.Get("/alerts/events", h.ListEvents)
	router.Get("/alerts/{id}", h.Get)
	router.Put("/alerts/{id}", h.Update)
	router.Delete("/alerts/{id}", h.Delete)
	router.Post("/alerts/{id}/evaluate", h.Evaluate)
}
