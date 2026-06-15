package usage

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/usages", h.Create)
	router.Post("/usages/bulk", h.CreateBulk)
	router.Post("/usages/search", h.Search)
	router.Post("/usageevents/prune", h.PruneEvents)
	router.Post("/usageevents/search", h.SearchEvents)
	router.Get("/usages/dimensions", h.ListDimensionValues)
	router.Get("/usages", h.List)
	router.Get("/usages/export", h.Export)
	router.Get("/usageingestions", h.ListIngestions)
	router.Get("/usageevents/prunes", h.ListPruneRuns)
	router.Get("/usageevents/export", h.ExportEvents)
	router.Get("/usageevents", h.ListEvents)
}
