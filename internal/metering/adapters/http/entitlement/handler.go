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

// PreviewPlan previews a proposed plan update.
//
// @Summary Preview plan update
// @ID previewPlan
// @Tags plans
// @Accept json
// @Produce json
// @Param id path string true "Plan ID"
// @Param request body PlanSaveRequest true "Plan"
// @Success 200 {object} PlanPreviewResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/plans/{id}/preview [post]
func (h *Handler) PreviewPlan(w http.ResponseWriter, r *http.Request) {
	var req PlanSaveRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}
	preview, err := h.service.PreviewPlan(r.Context(), appentitlement.UpdatePlanCommand{
		ID:          chi.URLParam(r, "id"),
		Name:        req.Name,
		Description: req.Description,
		Limits:      limitCommands(req.Limits),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, planPreviewResponse(preview))
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
	effectiveAt, err := request.OptionalTime("effective_at", req.EffectiveAt)
	if err != nil {
		respond.ValidationError(w, err)
		return
	}
	assignment, err := h.service.AssignSubject(r.Context(), appentitlement.AssignSubjectCommand{
		Subject:     chi.URLParam(r, "subject"),
		PlanID:      req.PlanID,
		EffectiveAt: effectiveAt,
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
// @Param include_history query bool false "Include ended assignments"
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
	includeHistory, err := request.ParseOptionalBool("include_history", r.URL.Query().Get("include_history"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}
	assignments, err := h.service.ListSubjectAssignments(r.Context(), appentitlement.AssignmentListQuery{
		Subject:        r.URL.Query().Get("subject"),
		PlanID:         r.URL.Query().Get("plan_id"),
		IncludeHistory: includeHistory,
		Limit:          limit,
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

// ListEntitlementStates lists current entitlement states.
//
// @Summary List entitlement states
// @ID listEntitlementStates
// @Tags entitlements
// @Produce json
// @Param subject query string false "Subject"
// @Param meter query string false "Meter"
// @Param state query string false "State"
// @Param limit query int false "Page size"
// @Success 200 {object} StateListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 403 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/entitlements/states [get]
func (h *Handler) ListEntitlementStates(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}
	states, err := h.service.ListEntitlementStates(r.Context(), appentitlement.StateListQuery{
		Subject:   r.URL.Query().Get("subject"),
		MeterName: r.URL.Query().Get("meter"),
		State:     appentitlement.OverageState(r.URL.Query().Get("state")),
		Limit:     limit,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	items := make([]StateResponse, 0, len(states.Items))
	for _, state := range states.Items {
		items = append(items, stateResponse(state))
	}
	respond.JSON(w, http.StatusOK, StateListResponse{Items: items})
}

// ListEntitlementEvents lists entitlement state transitions.
//
// @Summary List entitlement events
// @ID listEntitlementEvents
// @Tags entitlements
// @Produce json
// @Param subject query string false "Subject"
// @Param meter query string false "Meter"
// @Param plan_id query string false "Plan ID"
// @Param state query string false "State"
// @Param type query string false "Event type"
// @Param cursor query string false "Cursor"
// @Param limit query int false "Page size"
// @Success 200 {object} EventListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 403 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/entitlements/events [get]
func (h *Handler) ListEntitlementEvents(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}
	events, err := h.service.ListEntitlementEvents(r.Context(), appentitlement.EventListQuery{
		Subject:   r.URL.Query().Get("subject"),
		MeterName: r.URL.Query().Get("meter"),
		PlanID:    r.URL.Query().Get("plan_id"),
		State:     appentitlement.OverageState(r.URL.Query().Get("state")),
		Type:      appentitlement.EventType(r.URL.Query().Get("type")),
		Cursor:    r.URL.Query().Get("cursor"),
		Limit:     limit,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	items := make([]EventResponse, 0, len(events.Items))
	for _, event := range events.Items {
		items = append(items, eventResponse(event))
	}
	respond.JSON(w, http.StatusOK, EventListResponse{Items: items, NextCursor: events.NextCursor})
}

// ListEntitlementPeriodSnapshots lists quota period snapshots.
//
// @Summary List entitlement period snapshots
// @ID listEntitlementPeriodSnapshots
// @Tags entitlements
// @Produce json
// @Param subject query string false "Subject"
// @Param meter query string false "Meter"
// @Param plan_id query string false "Plan ID"
// @Param state query string false "State"
// @Param limit query int false "Page size"
// @Success 200 {object} PeriodSnapshotListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 403 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/entitlements/periods [get]
func (h *Handler) ListEntitlementPeriodSnapshots(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}
	snapshots, err := h.service.ListEntitlementPeriodSnapshots(r.Context(), appentitlement.SnapshotListQuery{
		Subject:   r.URL.Query().Get("subject"),
		MeterName: r.URL.Query().Get("meter"),
		PlanID:    r.URL.Query().Get("plan_id"),
		State:     appentitlement.OverageState(r.URL.Query().Get("state")),
		Limit:     limit,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	items := make([]PeriodSnapshotResponse, 0, len(snapshots.Items))
	for _, snapshot := range snapshots.Items {
		items = append(items, periodSnapshotResponse(snapshot))
	}
	respond.JSON(w, http.StatusOK, PeriodSnapshotListResponse{Items: items})
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
		ID:           result.Plan.ID,
		Name:         result.Plan.Name,
		Description:  result.Plan.Description,
		Version:      result.Plan.Version,
		ParentPlanID: result.Plan.ParentPlanID,
		IsCurrent:    result.Plan.IsCurrent,
		Limits:       limits,
		CreatedAt:    formatTime(result.Plan.CreatedAt),
		UpdatedAt:    formatTime(result.Plan.UpdatedAt),
	}
}

func planPreviewResponse(result appentitlement.PlanPreviewResult) PlanPreviewResponse {
	subjects := make([]PlanPreviewSubject, 0, len(result.Subjects))
	for _, subject := range result.Subjects {
		items := make([]PlanPreviewItem, 0, len(subject.Items))
		for _, item := range subject.Items {
			items = append(items, PlanPreviewItem{
				Meter:              item.MeterName,
				Period:             string(item.Period),
				Current:            item.Current,
				CurrentLimit:       item.CurrentLimit,
				ProposedLimit:      item.ProposedLimit,
				CurrentState:       string(item.CurrentState),
				ProposedState:      string(item.ProposedState),
				Remaining:          item.Remaining,
				Overage:            item.Overage,
				Percent:            item.Percent,
				WarningPercent:     item.WarningPercent,
				From:               formatTime(item.From),
				To:                 formatTime(item.To),
				PeriodResetAt:      formatTime(item.PeriodResetAt),
				Unit:               item.Unit,
				Aggregation:        string(item.Aggregation),
				EventCount:         item.EventCount,
				Removed:            item.Removed,
				ExistingLimitFound: item.ExistingLimitFound,
			})
		}
		subjects = append(subjects, PlanPreviewSubject{
			Subject:             subject.Subject,
			AssignmentID:        subject.AssignmentID,
			AssignmentStatus:    string(subject.AssignmentStatus),
			CurrentPlanID:       subject.CurrentPlanID,
			CurrentPlanVersion:  subject.CurrentPlanVersion,
			ProposedPlanID:      subject.ProposedPlanID,
			ProposedPlanVersion: subject.ProposedPlanVersion,
			Items:               items,
		})
	}
	return PlanPreviewResponse{
		Current:  planResponse(result.Current),
		Proposed: planResponse(result.Proposed),
		Summary: PlanPreviewSummary{
			Subjects:      result.Summary.Subjects,
			OK:            result.Summary.OK,
			Warning:       result.Summary.Warning,
			Exceeded:      result.Summary.Exceeded,
			RemovedLimits: result.Summary.RemovedLimits,
		},
		Subjects: subjects,
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
	status := assignment.StatusAt(time.Now().UTC())
	return AssignmentResponse{
		ID:             assignment.ID,
		Subject:        assignment.Subject,
		PlanID:         assignment.PlanID,
		PlanName:       assignment.PlanName,
		PlanVersion:    assignment.PlanVersion,
		Status:         string(status),
		Active:         status == appentitlement.AssignmentStatusActive,
		AssignedAt:     formatTime(assignment.AssignedAt),
		PeriodAnchorAt: formatTime(assignment.PeriodAnchorAt),
		UnassignedAt:   optionalTime(assignment.UnassignedAt),
		UpdatedAt:      formatTime(assignment.UpdatedAt),
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
			Overage:        item.Overage,
			Percent:        item.Percent,
			WarningPercent: item.WarningPercent,
			From:           formatTime(item.From),
			To:             formatTime(item.To),
			PeriodResetAt:  formatTime(item.PeriodResetAt),
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
		Allowed:           result.Allowed,
		State:             string(result.State),
		Subject:           result.Subject,
		Meter:             result.MeterName,
		Quantity:          result.Quantity,
		Current:           result.Current,
		Limit:             result.Limit,
		Remaining:         result.Remaining,
		Overage:           result.Overage,
		PlanID:            result.PlanID,
		PlanName:          result.PlanName,
		Period:            string(result.Period),
		From:              optionalTime(result.From),
		To:                optionalTime(result.To),
		PeriodResetAt:     optionalTime(result.PeriodResetAt),
		RetryAfterSeconds: result.RetryAfterSeconds,
		Message:           result.Message,
	}
}

func stateResponse(state appentitlement.EntitlementState) StateResponse {
	return StateResponse{
		Subject:        state.Subject,
		Meter:          state.MeterName,
		PlanID:         state.PlanID,
		PlanName:       state.PlanName,
		Period:         string(state.Period),
		State:          string(state.State),
		Current:        state.Current,
		Limit:          state.Limit,
		Remaining:      state.Remaining,
		WarningPercent: state.WarningPercent,
		Message:        state.Message,
		EvaluatedAt:    formatTime(state.EvaluatedAt),
		UpdatedAt:      formatTime(state.UpdatedAt),
	}
}

func eventResponse(event appentitlement.EntitlementEvent) EventResponse {
	return EventResponse{
		ID:             event.ID,
		Subject:        event.Subject,
		Meter:          event.MeterName,
		PlanID:         event.PlanID,
		PlanName:       event.PlanName,
		Period:         string(event.Period),
		PreviousState:  string(event.PreviousState),
		State:          string(event.State),
		Type:           string(event.Type),
		Current:        event.Current,
		Limit:          event.Limit,
		Remaining:      event.Remaining,
		WarningPercent: event.WarningPercent,
		Message:        event.Message,
		CreatedAt:      formatTime(event.CreatedAt),
	}
}

func periodSnapshotResponse(snapshot appentitlement.EntitlementPeriodSnapshot) PeriodSnapshotResponse {
	return PeriodSnapshotResponse{
		Subject:        snapshot.Subject,
		Meter:          snapshot.MeterName,
		PlanID:         snapshot.PlanID,
		PlanName:       snapshot.PlanName,
		PlanVersion:    snapshot.PlanVersion,
		Period:         string(snapshot.Period),
		From:           formatTime(snapshot.From),
		To:             formatTime(snapshot.To),
		State:          string(snapshot.State),
		Current:        snapshot.Current,
		Limit:          snapshot.Limit,
		Included:       snapshot.Included,
		Overage:        snapshot.Overage,
		Remaining:      snapshot.Remaining,
		WarningPercent: snapshot.WarningPercent,
		EventCount:     snapshot.EventCount,
		UpdatedAt:      formatTime(snapshot.UpdatedAt),
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
