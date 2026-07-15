package system

import (
	"net/http"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/request"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
)

type Handler struct {
	service appsystem.Service
}

// Reconcile performs a bounded, read-only quota consistency scan.
//
// @Summary Reconcile quota records
// @Description Checks recent consumption decisions and active quota counters against source usage without modifying data.
// @ID reconcileQuotaRecords
// @Tags system
// @Produce json
// @Param limit query int false "Maximum decisions and active counters to inspect" default(100) maximum(500)
// @Param lookback_hours query int false "Recent decision lookback in hours" default(24) maximum(720)
// @Success 200 {object} ReconciliationResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/system/reconciliation [get]
func (h *Handler) Reconcile(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, request.Code(err), request.Message(err))
		return
	}
	lookback, err := request.ParseLimit(r.URL.Query().Get("lookback_hours"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_lookback_hours", "lookback_hours must be a positive integer")
		return
	}
	result, err := h.service.Reconcile(r.Context(), appsystem.ReconciliationQuery{Limit: limit, LookbackHours: lookback})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, reconciliationResponseFromResult(result))
}

// RepairCounter previews or applies a targeted, audited quota counter repair.
//
// @Summary Repair a quota counter
// @Description Set dry_run=true to preview. Applying requires expected_updated_at from the preview and fails if the counter changed.
// @ID repairQuotaCounter
// @Tags system
// @Accept json
// @Produce json
// @Param request body CounterRepairRequest true "Repair target and concurrency guard"
// @Success 200 {object} CounterRepairResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/system/reconciliation/repairs [post]
func (h *Handler) RepairCounter(w http.ResponseWriter, r *http.Request) {
	var body CounterRepairRequest
	if err := request.DecodeJSON(r.Body, &body); err != nil {
		respond.Error(w, http.StatusBadRequest, request.Code(err), request.Message(err))
		return
	}
	if body.DryRun == nil {
		respond.Error(w, http.StatusBadRequest, "invalid_dry_run", "dry_run is required")
		return
	}
	periodStart, err := request.RequiredTime("period_start", body.PeriodStart)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, request.Code(err), request.Message(err))
		return
	}
	expectedUpdatedAt, err := request.OptionalTime("expected_updated_at", body.ExpectedUpdatedAt)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, request.Code(err), request.Message(err))
		return
	}
	result, err := h.service.RepairCounter(r.Context(), appsystem.RepairCounterCommand{
		CounterRepairTarget: appsystem.CounterRepairTarget{Subject: body.Subject, MeterName: body.Meter, Period: body.Period, PeriodStart: periodStart},
		DryRun:              *body.DryRun, ExpectedUpdatedAt: expectedUpdatedAt,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, counterRepairResponse(result))
}

// ListCounterRepairs lists durable quota counter repair previews and applications.
//
// @Summary List quota counter repairs
// @ID listQuotaCounterRepairs
// @Tags system
// @Produce json
// @Param limit query int false "Maximum audit records" default(50) maximum(200)
// @Success 200 {object} CounterRepairListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/system/reconciliation/repairs [get]
func (h *Handler) ListCounterRepairs(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, request.Code(err), request.Message(err))
		return
	}
	runs, err := h.service.ListCounterRepairRuns(r.Context(), limit)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	items := make([]CounterRepairResponse, 0, len(runs))
	for _, run := range runs {
		items = append(items, counterRepairResponse(run))
	}
	respond.JSON(w, http.StatusOK, CounterRepairListResponse{Items: items})
}

func reconciliationResponseFromResult(result appsystem.ReconciliationResult) ReconciliationResponse {
	issues := make([]ReconciliationIssueResponse, 0, len(result.Issues))
	for _, issue := range result.Issues {
		periodStart := ""
		if !issue.PeriodStart.IsZero() {
			periodStart = issue.PeriodStart.Format(time.RFC3339Nano)
		}
		issues = append(issues, ReconciliationIssueResponse{
			Kind: issue.Kind, Severity: issue.Severity, Subject: issue.Subject, Meter: issue.MeterName,
			IdempotencyKey: issue.IdempotencyKey, Period: issue.Period, PeriodStart: periodStart,
			Expected: issue.Expected, Actual: issue.Actual, Message: issue.Message,
		})
	}
	return ReconciliationResponse{
		Status: result.Status, DecisionsChecked: result.DecisionChecked, CountersChecked: result.CountersChecked,
		Issues: issues, Truncated: result.Truncated, LookbackHours: result.LookbackHours, CheckedAt: result.CheckedAt.Format(time.RFC3339),
	}
}

func counterRepairResponse(result appsystem.CounterRepairResult) CounterRepairResponse {
	return CounterRepairResponse{
		ID: result.ID, Subject: result.Subject, Meter: result.MeterName, Period: result.Period,
		PeriodStart: result.PeriodStart.Format(time.RFC3339Nano), PeriodEnd: result.PeriodEnd.Format(time.RFC3339Nano),
		DryRun: result.DryRun, Applied: result.Applied, Before: counterSnapshotResponse(result.Before), After: counterSnapshotResponse(result.After),
		CounterUpdatedAt: result.CounterUpdatedAt.Format(time.RFC3339Nano), CreatedAt: result.CreatedAt.Format(time.RFC3339Nano),
	}
}

func counterSnapshotResponse(snapshot appsystem.CounterSnapshot) CounterSnapshotResponse {
	first, last := "", ""
	if !snapshot.FirstEventTime.IsZero() {
		first = snapshot.FirstEventTime.Format(time.RFC3339Nano)
	}
	if !snapshot.LastEventTime.IsZero() {
		last = snapshot.LastEventTime.Format(time.RFC3339Nano)
	}
	return CounterSnapshotResponse{
		EventCount: snapshot.EventCount, QuantitySum: snapshot.QuantitySum, QuantityMin: snapshot.QuantityMin, QuantityMax: snapshot.QuantityMax,
		FirstQuantity: snapshot.FirstQuantity, FirstEventTime: first, LastQuantity: snapshot.LastQuantity, LastEventTime: last,
	}
}

func NewHandler(service appsystem.Service) *Handler {
	return &Handler{service: service}
}

// Stats returns operational system stats.
//
// @Summary Get system stats
// @ID getSystemStats
// @Tags system
// @Produce json
// @Success 200 {object} StatsResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/system/stats [get]
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.Stats(r.Context())
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, statsResponseFromResult(stats))
}

func statsResponseFromResult(stats appsystem.StatsResult) StatsResponse {
	var lastPruneRun *LastPruneRunResponse
	if stats.LastPruneRun.ID != "" {
		lastPruneRun = &LastPruneRunResponse{
			ID:        stats.LastPruneRun.ID,
			Deleted:   stats.LastPruneRun.Deleted,
			DryRun:    stats.LastPruneRun.DryRun,
			CreatedAt: stats.LastPruneRun.CreatedAt.Format(time.RFC3339),
		}
	}
	var lastDecisionPruneRun *LastDecisionPruneRunResponse
	if stats.LastDecisionPruneRun.ID != "" {
		lastDecisionPruneRun = &LastDecisionPruneRunResponse{
			ID: stats.LastDecisionPruneRun.ID, Before: stats.LastDecisionPruneRun.Before.Format(time.RFC3339),
			Deleted: stats.LastDecisionPruneRun.Deleted, DryRun: stats.LastDecisionPruneRun.DryRun,
			CreatedAt: stats.LastDecisionPruneRun.CreatedAt.Format(time.RFC3339),
		}
	}

	return StatsResponse{
		Meters:               stats.Meters,
		UsageEvents:          stats.UsageEvents,
		PruneRuns:            stats.PruneRuns,
		LastPruneRun:         lastPruneRun,
		ConsumptionDecisions: stats.ConsumptionDecisions,
		DecisionPruneRuns:    stats.DecisionPruneRuns,
		LastDecisionPruneRun: lastDecisionPruneRun,
	}
}
