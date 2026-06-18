package alert

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/request"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
	appalert "github.com/ssubedir/open-spanner/internal/metering/app/alert"
)

type Handler struct {
	service appalert.Service
}

func NewHandler(service appalert.Service) *Handler {
	return &Handler{service: service}
}

// Create creates an alert rule.
//
// @Summary Create alert rule
// @ID createAlertRule
// @Tags alerts
// @Accept json
// @Produce json
// @Param request body SaveRequest true "Alert rule"
// @Success 201 {object} RuleResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/alerts [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req SaveRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}

	rule, err := h.service.Create(r.Context(), appalert.SaveCommand{
		Name:               req.Name,
		MeterName:          req.Meter,
		Enabled:            req.Enabled,
		Subject:            req.Subject,
		Metadata:           req.Metadata,
		Window:             secondsDuration(req.WindowSeconds),
		Comparator:         req.Comparator,
		Threshold:          req.Threshold,
		EvaluationInterval: secondsDuration(req.EvaluationIntervalSeconds),
		TriggerType:        req.TriggerType,
		WebhookURL:         req.WebhookURL,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, ruleResponse(rule))
}

// List lists alert rules.
//
// @Summary List alert rules
// @ID listAlertRules
// @Tags alerts
// @Produce json
// @Param meter query string false "Meter name"
// @Param enabled query bool false "Enabled filter"
// @Param limit query int false "Result limit"
// @Success 200 {object} RuleListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/alerts [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}
	enabled, err := enabledFilter(r.URL.Query().Get("enabled"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	rules, err := h.service.List(r.Context(), appalert.ListQuery{
		MeterName: r.URL.Query().Get("meter"),
		Enabled:   enabled,
		Limit:     limit,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	res := make([]RuleResponse, 0, len(rules.Items))
	for _, rule := range rules.Items {
		res = append(res, ruleResponse(rule))
	}
	respond.JSON(w, http.StatusOK, RuleListResponse{Items: res})
}

// Get gets an alert rule.
//
// @Summary Get alert rule
// @ID getAlertRule
// @Tags alerts
// @Produce json
// @Param id path string true "Alert rule ID"
// @Success 200 {object} RuleResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/alerts/{id} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	rule, err := h.service.Get(r.Context(), appalert.GetQuery{ID: chi.URLParam(r, "id")})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, ruleResponse(rule))
}

// Update updates an alert rule.
//
// @Summary Update alert rule
// @ID updateAlertRule
// @Tags alerts
// @Accept json
// @Produce json
// @Param id path string true "Alert rule ID"
// @Param request body UpdateRequest true "Alert rule update"
// @Success 200 {object} RuleResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/alerts/{id} [put]
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var req UpdateRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}

	rule, err := h.service.Update(r.Context(), appalert.UpdateCommand{
		ID:                 chi.URLParam(r, "id"),
		Name:               stringValue(req.Name),
		MeterName:          stringValue(req.Meter),
		Enabled:            req.Enabled,
		Subject:            req.Subject,
		Metadata:           req.Metadata,
		Window:             secondsDurationPointer(req.WindowSeconds),
		Comparator:         stringValue(req.Comparator),
		Threshold:          req.Threshold,
		EvaluationInterval: secondsDurationPointer(req.EvaluationIntervalSeconds),
		TriggerType:        stringValue(req.TriggerType),
		WebhookURL:         req.WebhookURL,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, ruleResponse(rule))
}

// Delete deletes an alert rule.
//
// @Summary Delete alert rule
// @ID deleteAlertRule
// @Tags alerts
// @Produce json
// @Param id path string true "Alert rule ID"
// @Success 204
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/alerts/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), appalert.DeleteCommand{ID: chi.URLParam(r, "id")}); err != nil {
		respond.ServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Evaluate evaluates an alert rule immediately.
//
// @Summary Evaluate alert rule
// @ID evaluateAlertRule
// @Tags alerts
// @Produce json
// @Param id path string true "Alert rule ID"
// @Success 200 {object} EvaluationResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/alerts/{id}/evaluate [post]
func (h *Handler) Evaluate(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Evaluate(r.Context(), appalert.EvaluateCommand{RuleID: chi.URLParam(r, "id")})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	response := EvaluationResponse{
		Rule:  ruleResponse(result.Rule),
		State: stateResponse(result.State),
	}
	if result.Event != nil {
		event := eventResponse(*result.Event)
		response.Event = &event
	}
	respond.JSON(w, http.StatusOK, response)
}

// ListEvents lists alert rule events.
//
// @Summary List alert events
// @ID listAlertEvents
// @Tags alerts
// @Produce json
// @Param rule_id query string false "Alert rule ID"
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} EventListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/alerts/events [get]
func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	events, err := h.service.ListEvents(r.Context(), appalert.EventListQuery{
		RuleID: r.URL.Query().Get("rule_id"),
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	res := make([]EventResponse, 0, len(events.Items))
	for _, event := range events.Items {
		res = append(res, eventResponse(event))
	}
	respond.JSON(w, http.StatusOK, EventListResponse{Items: res, NextCursor: events.NextCursor})
}

func ruleResponse(rule appalert.RuleResult) RuleResponse {
	response := RuleResponse{
		ID:                        rule.ID,
		Name:                      rule.Name,
		Meter:                     rule.MeterName,
		Enabled:                   rule.Enabled,
		Subject:                   rule.Subject,
		Metadata:                  rule.Metadata,
		WindowSeconds:             rule.WindowSeconds,
		Comparator:                rule.Comparator,
		Threshold:                 rule.Threshold,
		EvaluationIntervalSeconds: rule.EvaluationInterval,
		TriggerType:               rule.TriggerType,
		WebhookURL:                rule.WebhookURL,
		NextEvaluateAt:            formatTime(rule.NextEvaluateAt),
		CreatedAt:                 formatTime(rule.CreatedAt),
		UpdatedAt:                 formatTime(rule.UpdatedAt),
	}
	if rule.State != nil {
		state := stateResponse(*rule.State)
		response.State = &state
	}
	return response
}

func stateResponse(state appalert.StateResult) StateResponse {
	return StateResponse{
		Status:      state.Status,
		Value:       state.Value,
		Message:     state.Message,
		EvaluatedAt: formatTime(state.EvaluatedAt),
		UpdatedAt:   formatTime(state.UpdatedAt),
	}
}

func eventResponse(event appalert.EventResult) EventResponse {
	return EventResponse{
		ID:        event.ID,
		RuleID:    event.RuleID,
		Type:      event.Type,
		Value:     event.Value,
		Message:   event.Message,
		CreatedAt: formatTime(event.CreatedAt),
	}
}

func enabledFilter(value string) (*bool, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, request.NewValidationError("invalid_enabled", "enabled must be true or false")
	}
	return &parsed, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func secondsDuration(value int) time.Duration {
	if value <= 0 {
		return 0
	}
	return time.Duration(value) * time.Second
}

func secondsDurationPointer(value *int) time.Duration {
	if value == nil {
		return 0
	}
	return secondsDuration(*value)
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
