package meter

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/access"
	appmeter "github.com/ssubedir/open-spanner/internal/metering/app/meter"
)

func (h *Handler) RegisterRoutes(router chi.Router, authorizer access.Authorizer) {
	routes := access.NewRouter(router, authorizer)

	routes.Post("/meters", h.Create, access.MetersWrite(createMeterResource))
	routes.Get("/meters", h.List, access.MetersRead(meterListResource))
	routes.Get("/meters/stats", h.ListStats, access.MetersRead(allMetersResource))
	routes.Get("/meters/{id}", h.Get, access.MetersRead(h.meterByIDResource))
	routes.Put("/meters/{id}", h.Update, access.MetersWrite(h.meterByIDResource))
	routes.Delete("/meters/{id}", h.Delete, access.MetersWrite(h.meterByIDResource))
}

var (
	allMetersResource   = access.Static(access.Meter(""))
	createMeterResource = access.JSONBodyResource(func(req CreateRequest) (access.Resource, error) {
		return access.Meter(req.Name), nil
	})
)

func meterListResource(r *http.Request) ([]access.Resource, error) {
	return access.Resources(access.Meter(r.URL.Query().Get("name"))), nil
}

func (h *Handler) meterByIDResource(r *http.Request) ([]access.Resource, error) {
	meter, err := h.service.Get(r.Context(), appmeter.GetQuery{ID: chi.URLParam(r, "id")})
	if err != nil {
		return nil, err
	}
	return access.Resources(access.MeterByID(meter.ID, meter.Name)), nil
}
