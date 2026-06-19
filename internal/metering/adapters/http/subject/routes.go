package subject

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/access"
)

func (h *Handler) RegisterRoutes(router chi.Router, authorizer access.Authorizer) {
	routes := access.NewRouter(router, authorizer)

	routes.Get("/subjects", h.List, access.UsageRead(allUsageResource))
	routes.Get("/subjects/{subject}/usageevents", h.ListEvents, access.UsageRead(subjectUsageResource))
}

var allUsageResource = access.Static(access.Usage("", ""))

func subjectUsageResource(r *http.Request) ([]access.Resource, error) {
	return access.Resources(access.Usage("", chi.URLParam(r, "subject"))), nil
}
