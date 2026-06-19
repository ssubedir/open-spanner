package alert

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/access"
	appalert "github.com/ssubedir/open-spanner/internal/metering/app/alert"
)

func (h *Handler) RegisterRoutes(router chi.Router, authorizer access.Authorizer) {
	routes := access.NewRouter(router, authorizer)

	routes.Route("/alerts", func(r access.Router) {
		r.Get("/", h.List, access.AlertsRead(alertListResource))
		r.Post("/", h.Create, access.AlertsWrite(createAlertResource))
		r.Get("/events", h.ListEvents, access.AlertsRead(h.alertEventResource))

		r.Route("/destinations", func(destinations access.Router) {
			destinations.Get("/", h.ListDestinations, access.AlertsRead(alertDestinationResource))
			destinations.Post("/", h.CreateDestination, access.AlertsWrite(alertDestinationResource))
			destinations.Put("/{id}", h.UpdateDestination, access.AlertsWrite(alertDestinationResource))
			destinations.Delete("/{id}", h.DeleteDestination, access.AlertsWrite(alertDestinationResource))
			destinations.Post("/{id}/webhook-secret/rotate", h.RotateDestinationSecret, access.AlertsWrite(alertDestinationResource))
		})

		r.Get("/{id}", h.Get, access.AlertsRead(h.alertRuleByIDResource))
		r.Put("/{id}", h.Update, access.AlertsWrite(h.alertRuleUpdateResource))
		r.Delete("/{id}", h.Delete, access.AlertsWrite(h.alertRuleByIDResource))
		r.Post("/{id}/evaluate", h.Evaluate, access.AlertsWrite(h.alertRuleByIDResource))
	})
}

var (
	alertDestinationResource = access.Static(access.Alert(""))
	createAlertResource      = access.JSONBodyResource(func(req SaveRequest) (access.Resource, error) {
		return access.Alert(req.Meter), nil
	})
)

func alertListResource(r *http.Request) ([]access.Resource, error) {
	return access.Resources(access.Alert(r.URL.Query().Get("meter"))), nil
}

func (h *Handler) alertEventResource(r *http.Request) ([]access.Resource, error) {
	ruleID := r.URL.Query().Get("rule_id")
	if ruleID == "" {
		return access.Resources(access.Alert("")), nil
	}
	return h.alertRuleResource(r, ruleID)
}

func (h *Handler) alertRuleByIDResource(r *http.Request) ([]access.Resource, error) {
	return h.alertRuleResource(r, chi.URLParam(r, "id"))
}

func (h *Handler) alertRuleUpdateResource(r *http.Request) ([]access.Resource, error) {
	return access.JSONBodyRequest(func(r *http.Request, req UpdateRequest) ([]access.Resource, error) {
		resources, err := h.alertRuleResource(r, chi.URLParam(r, "id"))
		if err != nil {
			return nil, err
		}
		if req.Meter != nil && *req.Meter != "" && *req.Meter != resources[0].Meter {
			resources = append(resources, access.Alert(*req.Meter))
		}
		return resources, nil
	})(r)
}

func (h *Handler) alertRuleResource(r *http.Request, id string) ([]access.Resource, error) {
	rule, err := h.service.Get(r.Context(), appalert.GetQuery{ID: id})
	if err != nil {
		return nil, err
	}
	return access.Resources(access.AlertByID(rule.ID, rule.MeterName)), nil
}
