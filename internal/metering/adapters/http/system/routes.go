package system

import (
	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/access"
)

func (h *Handler) RegisterRoutes(router chi.Router, authorizer access.Authorizer) {
	routes := access.NewRouter(router, authorizer)

	routes.Get("/system/stats", h.Stats, access.SystemRead(systemResource))
	routes.Get("/system/reconciliation", h.Reconcile, access.SystemRead(systemResource))
	routes.Get("/system/reconciliation/runs", h.ListReconciliationRuns, access.SystemRead(systemResource))
	routes.Get("/system/reconciliation/notifications", h.ListReconciliationNotifications, access.SystemRead(systemResource))
	routes.Post("/system/reconciliation/notifications/{id}/retry", h.RequeueReconciliationNotification, access.SystemWrite(systemResource))
	routes.Get("/system/reconciliation/repairs", h.ListCounterRepairs, access.SystemRead(systemResource))
	routes.Post("/system/reconciliation/repairs", h.RepairCounter, access.SystemWrite(systemResource))
}

var systemResource = access.Static(access.System())
