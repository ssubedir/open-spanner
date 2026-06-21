package entitlement

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/request"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
	appentitlement "github.com/ssubedir/open-spanner/internal/metering/app/entitlement"
)

type Handler struct {
	service appentitlement.Service
}

func NewHandler(service appentitlement.Service) *Handler {
	return &Handler{service: service}
}

// CreatePlan creates a plan.
//
// @Summary Create plan
// @ID createPlan
// @Tags plans
// @Accept json
// @Produce json
// @Param request body PlanSaveRequest true "Plan"
// @Success 201 {object} PlanResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/plans [post]
func (h *Handler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	var req PlanSaveRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}
	plan, err := h.service.CreatePlan(r.Context(), appentitlement.SavePlanCommand{
		Name:        req.Name,
		Description: req.Description,
		Limits:      limitCommands(req.Limits),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, planResponse(plan))
}

// ListPlans lists plans.
//
// @Summary List plans
// @ID listPlans
// @Tags plans
// @Produce json
// @Param limit query int false "Page size"
// @Success 200 {object} PlanListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/plans [get]
func (h *Handler) ListPlans(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}
	plans, err := h.service.ListPlans(r.Context(), appentitlement.ListPlansQuery{Limit: limit})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	items := make([]PlanResponse, 0, len(plans.Items))
	for _, item := range plans.Items {
		items = append(items, planResponse(item))
	}
	respond.JSON(w, http.StatusOK, PlanListResponse{Items: items})
}

// GetPlan gets a plan.
//
// @Summary Get plan
// @ID getPlan
// @Tags plans
// @Produce json
// @Param id path string true "Plan ID"
// @Success 200 {object} PlanResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/plans/{id} [get]
func (h *Handler) GetPlan(w http.ResponseWriter, r *http.Request) {
	plan, err := h.service.GetPlan(r.Context(), appentitlement.GetPlanQuery{ID: chi.URLParam(r, "id")})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, planResponse(plan))
}

// UpdatePlan updates a plan and replaces its limits.
//
// @Summary Update plan
// @ID updatePlan
// @Tags plans
// @Accept json
// @Produce json
// @Param id path string true "Plan ID"
// @Param request body PlanSaveRequest true "Plan"
// @Success 200 {object} PlanResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/plans/{id} [put]
func (h *Handler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	var req PlanSaveRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}
	plan, err := h.service.UpdatePlan(r.Context(), appentitlement.UpdatePlanCommand{
		ID:          chi.URLParam(r, "id"),
		Name:        req.Name,
		Description: req.Description,
		Limits:      limitCommands(req.Limits),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, planResponse(plan))
}

// DeletePlan deletes a plan.
//
// @Summary Delete plan
// @ID deletePlan
// @Tags plans
// @Produce json
// @Param id path string true "Plan ID"
// @Success 204
// @Failure 404 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/plans/{id} [delete]
func (h *Handler) DeletePlan(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeletePlan(r.Context(), appentitlement.DeletePlanCommand{ID: chi.URLParam(r, "id")}); err != nil {
		respond.ServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AssignSubject assigns a subject to a plan.
//
// @Summary Assign subject to plan
// @ID assignSubjectPlan
// @Tags plans
// @Accept json
// @Produce json
// @Param subject path string true "Subject"
// @Param request body AssignmentRequest true "Assignment"
// @Success 200 {object} AssignmentResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/plans/subjects/{subject} [put]
func (h *Handler) AssignSubject(w http.ResponseWriter, r *http.Request) {
	var req AssignmentRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}
	assignment, err := h.service.AssignSubject(r.Context(), appentitlement.AssignSubjectCommand{
		Subject: chi.URLParam(r, "subject"),
		PlanID:  req.PlanID,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, assignmentResponse(assignment.Assignment))
}

// DeleteSubjectAssignment removes a subject plan assignment.
//
// @Summary Remove subject plan assignment
// @ID deleteSubjectPlanAssignment
// @Tags plans
// @Produce json
// @Param subject path string true "Subject"
// @Success 204
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/plans/subjects/{subject} [delete]
func (h *Handler) DeleteSubjectAssignment(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteSubjectAssignment(r.Context(), appentitlement.DeleteSubjectAssignmentCommand{Subject: chi.URLParam(r, "subject")}); err != nil {
		respond.ServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListSubjectAssignments lists subject plan assignments.
//
// @Summary List subject plan assignments
// @ID listSubjectPlanAssignments
// @Tags plans
// @Produce json
// @Param subject query string false "Subject"
// @Param plan_id query string false "Plan ID"
// @Param limit query int false "Page size"
// @Success 200 {object} AssignmentListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/plans/assignments [get]
func (h *Handler) ListSubjectAssignments(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}
	assignments, err := h.service.ListSubjectAssignments(r.Context(), appentitlement.AssignmentListQuery{
		Subject: r.URL.Query().Get("subject"),
		PlanID:  r.URL.Query().Get("plan_id"),
		Limit:   limit,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	items := make([]AssignmentResponse, 0, len(assignments.Items))
	for _, item := range assignments.Items {
		items = append(items, assignmentResponse(item))
	}
	respond.JSON(w, http.StatusOK, AssignmentListResponse{Items: items})
}

// GetSubjectProgress gets usage progress against a subject's plan.
//
// @Summary Get subject plan progress
// @ID getSubjectPlanProgress
// @Tags plans
// @Produce json
// @Param subject path string true "Subject"
// @Success 200 {object} ProgressResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/plans/subjects/{subject}/progress [get]
func (h *Handler) GetSubjectProgress(w http.ResponseWriter, r *http.Request) {
	progress, err := h.service.GetSubjectProgress(r.Context(), appentitlement.SubjectProgressQuery{Subject: chi.URLParam(r, "subject")})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, progressResponse(progress))
}

// CheckEntitlement checks whether a subject has quota for a meter.
//
// @Summary Check entitlement quota
// @ID checkEntitlement
// @Tags plans
// @Accept json
// @Produce json
// @Param request body CheckRequest true "Entitlement check"
// @Success 200 {object} CheckResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 403 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/entitlements/check [post]
func (h *Handler) CheckEntitlement(w http.ResponseWriter, r *http.Request) {
	var req CheckRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}
	result, err := h.service.Check(r.Context(), appentitlement.CheckCommand{
		Subject:  req.Subject,
		Meter:    req.Meter,
		Quantity: req.Quantity,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, checkResponse(result))
}

func limitCommands(input []LimitRequest) []appentitlement.LimitCommand {
	limits := make([]appentitlement.LimitCommand, 0, len(input))
	for _, limit := range input {
		limits = append(limits, appentitlement.LimitCommand{
			MeterName:      limit.Meter,
			Period:         limit.Period,
			Limit:          limit.Limit,
			WarningPercent: limit.WarningPercent,
		})
	}
	return limits
}

func planResponse(result appentitlement.PlanResult) PlanResponse {
	limits := make([]LimitResponse, 0, len(result.Limits))
	for _, limit := range result.Limits {
		limits = append(limits, limitResponse(limit))
	}
	return PlanResponse{
		ID:          result.Plan.ID,
		Name:        result.Plan.Name,
		Description: result.Plan.Description,
		Limits:      limits,
		CreatedAt:   formatTime(result.Plan.CreatedAt),
		UpdatedAt:   formatTime(result.Plan.UpdatedAt),
	}
}

func limitResponse(limit appentitlement.PlanLimit) LimitResponse {
	return LimitResponse{
		ID:             limit.ID,
		Meter:          limit.MeterName,
		Period:         string(limit.Period),
		Limit:          limit.Limit,
		WarningPercent: limit.WarningPercent,
		CreatedAt:      formatTime(limit.CreatedAt),
		UpdatedAt:      formatTime(limit.UpdatedAt),
	}
}

func assignmentResponse(assignment appentitlement.SubjectAssignment) AssignmentResponse {
	return AssignmentResponse{
		Subject:    assignment.Subject,
		PlanID:     assignment.PlanID,
		PlanName:   assignment.PlanName,
		AssignedAt: formatTime(assignment.AssignedAt),
		UpdatedAt:  formatTime(assignment.UpdatedAt),
	}
}

func progressResponse(result appentitlement.SubjectProgressResult) ProgressResponse {
	items := make([]ProgressItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, ProgressItemResponse{
			Meter:          item.MeterName,
			Period:         string(item.Period),
			State:          string(item.State),
			Current:        item.Current,
			Limit:          item.Limit,
			Remaining:      item.Remaining,
			Percent:        item.Percent,
			WarningPercent: item.WarningPercent,
			From:           formatTime(item.From),
			To:             formatTime(item.To),
			Unit:           item.Unit,
			Aggregation:    string(item.Aggregation),
		})
	}
	return ProgressResponse{
		Subject: result.Subject,
		Plan:    planResponse(result.Plan),
		Items:   items,
	}
}

func checkResponse(result appentitlement.EntitlementCheckResult) CheckResponse {
	return CheckResponse{
		Allowed:   result.Allowed,
		State:     string(result.State),
		Subject:   result.Subject,
		Meter:     result.MeterName,
		Quantity:  result.Quantity,
		Current:   result.Current,
		Limit:     result.Limit,
		Remaining: result.Remaining,
		PlanID:    result.PlanID,
		PlanName:  result.PlanName,
		Period:    string(result.Period),
		From:      optionalTime(result.From),
		To:        optionalTime(result.To),
		Message:   result.Message,
	}
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func optionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return formatTime(value)
}
