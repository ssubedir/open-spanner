package entitlement

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/access"
	appentitlement "github.com/ssubedir/open-spanner/internal/metering/app/entitlement"
)

func (h *Handler) RegisterRoutes(router chi.Router, authorizer access.Authorizer) {
	routes := access.NewRouter(router, authorizer)

	routes.Route("/plans", func(r access.Router) {
		r.Get("/", h.ListPlans, access.PlansRead(allPlansResource))
		r.Post("/", h.CreatePlan, access.PlansWrite(planSaveResource))
		r.Get("/assignments", h.ListSubjectAssignments, access.PlansRead(allPlansResource))
		r.Put("/subjects/{subject}", h.AssignSubject, access.PlansWrite(allPlansResource))
		r.Delete("/subjects/{subject}", h.DeleteSubjectAssignment, access.PlansWrite(allPlansResource))
		r.Get("/subjects/{subject}/progress", h.GetSubjectProgress, access.PlansRead(allPlansResource))
		r.Get("/{id}", h.GetPlan, access.PlansRead(h.planByIDResource))
		r.Put("/{id}", h.UpdatePlan, access.PlansWrite(h.planUpdateResource))
		r.Delete("/{id}", h.DeletePlan, access.PlansWrite(h.planByIDResource))
	})

	routes.Post("/entitlements/check", h.CheckEntitlement, access.PlansRead(entitlementCheckResource))
}

var (
	allPlansResource = access.Static(access.Plan(""))
	planSaveResource = access.JSONBody(func(req PlanSaveRequest) ([]access.Resource, error) {
		resources := resourcesForLimits(req.Limits)
		if len(resources) == 0 {
			resources = append(resources, access.Plan(""))
		}
		return resources, nil
	})
	entitlementCheckResource = access.JSONBodyResource(func(req CheckRequest) (access.Resource, error) {
		return access.Plan(req.Meter), nil
	})
)

func (h *Handler) planByIDResource(r *http.Request) ([]access.Resource, error) {
	plan, err := h.service.GetPlan(r.Context(), appentitlement.GetPlanQuery{ID: chi.URLParam(r, "id")})
	if err != nil {
		return nil, err
	}
	resources := resourcesForPlan(plan)
	if len(resources) == 0 {
		resources = append(resources, access.PlanByID(plan.Plan.ID, ""))
	}
	return resources, nil
}

func (h *Handler) planUpdateResource(r *http.Request) ([]access.Resource, error) {
	return access.JSONBodyRequest(func(r *http.Request, req PlanSaveRequest) ([]access.Resource, error) {
		existing, err := h.planByIDResource(r)
		if err != nil {
			return nil, err
		}
		next := resourcesForLimits(req.Limits)
		resources := append(existing, next...)
		if len(resources) == 0 {
			resources = append(resources, access.Plan(""))
		}
		return resources, nil
	})(r)
}

func resourcesForPlan(plan appentitlement.PlanResult) []access.Resource {
	resources := make([]access.Resource, 0, len(plan.Limits))
	for _, limit := range plan.Limits {
		resources = append(resources, access.PlanByID(plan.Plan.ID, limit.MeterName))
	}
	return resources
}

func resourcesForLimits(limits []LimitRequest) []access.Resource {
	resources := make([]access.Resource, 0, len(limits))
	seen := map[string]struct{}{}
	for _, limit := range limits {
		if _, ok := seen[limit.Meter]; ok {
			continue
		}
		seen[limit.Meter] = struct{}{}
		resources = append(resources, access.Plan(limit.Meter))
	}
	return resources
}
