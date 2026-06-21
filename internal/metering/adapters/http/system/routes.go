package system

import (
	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/access"
)

func (h *Handler) RegisterRoutes(router chi.Router, authorizer access.Authorizer) {
	routes := access.NewRouter(router, authorizer)

	routes.Get("/system/stats", h.Stats, access.SystemRead(systemResource))
}

var systemResource = access.Static(access.System())
