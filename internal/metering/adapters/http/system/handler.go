package system

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/request"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
)

type Handler struct {
	service appsystem.Service
}

// ListWorkerDeadLetters lists durable alert and entitlement worker failures.
//
// @Summary List worker dead letters
// @ID listWorkerDeadLetters
// @Tags system
// @Produce json
// @Param limit query int false "Maximum audit records" default(50) maximum(200)
// @Success 200 {object} WorkerDeadLetterListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/system/workers/dead-letters [get]
func (h *Handler) ListWorkerDeadLetters(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, request.Code(err), request.Message(err))
		return
	}
	items, err := h.service.ListWorkerDeadLetters(r.Context(), limit)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	response := WorkerDeadLetterListResponse{Items: make([]WorkerDeadLetterResponse, 0, len(items))}
	for _, item := range items {
		converted := WorkerDeadLetterResponse{ID: item.ID, WorkerName: item.WorkerName, JobKey: item.JobKey, RuleID: item.RuleID, Subject: item.Subject, MeterName: item.MeterName, Attempts: item.Attempts, LastError: item.LastError, Status: item.Status, CreatedAt: item.CreatedAt.Format(time.RFC3339Nano)}
		if !item.RequeuedAt.IsZero() {
			converted.RequeuedAt = item.RequeuedAt.Format(time.RFC3339Nano)
		}
		response.Items = append(response.Items, converted)
	}
	respond.JSON(w, http.StatusOK, response)
}

// RetryWorkerDeadLetter recreates a failed worker job and preserves its audit record.
//
// @Summary Retry a worker dead letter
// @ID retryWorkerDeadLetter
// @Tags system
// @Produce json
// @Param id path string true "Dead-letter ID"
// @Success 204
// @Failure 404 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/system/workers/dead-letters/{id}/retry [post]
func (h *Handler) RetryWorkerDeadLetter(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RetryWorkerDeadLetter(r.Context(), chi.URLParam(r, "id")); err != nil {
		respond.ServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

// ListReconciliationRuns lists durable scheduled reconciliation outcomes.
//
// @Summary List reconciliation runs
// @ID listReconciliationRuns
// @Tags system
// @Produce json
// @Param limit query int false "Maximum run records" default(50) maximum(200)
// @Success 200 {object} ReconciliationRunListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/system/reconciliation/runs [get]
func (h *Handler) ListReconciliationRuns(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, request.Code(err), request.Message(err))
		return
	}
	runs, err := h.service.ListReconciliationRuns(r.Context(), limit)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	items := make([]ReconciliationRunResponse, 0, len(runs))
	for _, run := range runs {
		items = append(items, reconciliationRunResponse(run))
	}
	respond.JSON(w, http.StatusOK, ReconciliationRunListResponse{Items: items})
}

// ListReconciliationNotifications lists durable reconciliation webhook deliveries.
//
// @Summary List reconciliation notifications
// @ID listReconciliationNotifications
// @Tags system
// @Produce json
// @Param limit query int false "Maximum notification records" default(50) maximum(200)
// @Success 200 {object} ReconciliationNotificationListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/system/reconciliation/notifications [get]
func (h *Handler) ListReconciliationNotifications(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, request.Code(err), request.Message(err))
		return
	}
	notifications, err := h.service.ListReconciliationNotifications(r.Context(), limit)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	items := make([]ReconciliationNotificationResponse, 0, len(notifications))
	for _, notification := range notifications {
		deliveredAt := ""
		if !notification.DeliveredAt.IsZero() {
			deliveredAt = notification.DeliveredAt.Format(time.RFC3339Nano)
		}
		attempts := make([]ReconciliationNotificationAttemptResponse, 0, len(notification.AttemptHistory))
		for _, attempt := range notification.AttemptHistory {
			attempts = append(attempts, ReconciliationNotificationAttemptResponse{Attempt: attempt.Attempt, Status: attempt.Status, Error: attempt.Error, CreatedAt: attempt.CreatedAt.Format(time.RFC3339Nano)})
		}
		items = append(items, ReconciliationNotificationResponse{ID: notification.ID, EventType: notification.EventType, Status: notification.Status, Attempts: notification.Attempts, TotalAttempts: notification.TotalAttempts, NextAttemptAt: notification.NextAttemptAt.Format(time.RFC3339Nano), LastError: notification.LastError, CreatedAt: notification.CreatedAt.Format(time.RFC3339Nano), DeliveredAt: deliveredAt, AttemptHistory: attempts})
	}
	respond.JSON(w, http.StatusOK, ReconciliationNotificationListResponse{Items: items})
}

// RequeueReconciliationNotification retries a dead-letter reconciliation notification.
//
// @Summary Retry reconciliation notification
// @ID retryReconciliationNotification
// @Tags system
// @Param id path string true "Notification ID"
// @Success 204
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/system/reconciliation/notifications/{id}/retry [post]
func (h *Handler) RequeueReconciliationNotification(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RequeueReconciliationNotification(r.Context(), chi.URLParam(r, "id")); err != nil {
		respond.ServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func reconciliationRunResponse(run appsystem.ReconciliationRun) ReconciliationRunResponse {
	issues := reconciliationResponseFromResult(appsystem.ReconciliationResult{Issues: run.Issues}).Issues
	return ReconciliationRunResponse{ID: run.ID, Status: run.Status, DecisionsChecked: run.DecisionsChecked, CountersChecked: run.CountersChecked, IssueCount: run.IssueCount, Truncated: run.Truncated, LookbackHours: run.LookbackHours, DurationMS: run.Duration.Milliseconds(), Fingerprint: run.Fingerprint, Issues: issues, Error: run.Error, CreatedAt: run.CreatedAt.Format(time.RFC3339Nano)}
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
	var lastExportCleanupRun *LastExportCleanupRunResponse
	if stats.LastExportCleanupRun.ID != "" {
		lastExportCleanupRun = &LastExportCleanupRunResponse{ID: stats.LastExportCleanupRun.ID, ExpiredBefore: stats.LastExportCleanupRun.ExpiredBefore.Format(time.RFC3339), FilesDeleted: stats.LastExportCleanupRun.FilesDeleted, BytesReclaimed: stats.LastExportCleanupRun.BytesReclaimed, Failures: stats.LastExportCleanupRun.Failures, CreatedAt: stats.LastExportCleanupRun.CreatedAt.Format(time.RFC3339)}
	}
	if stats.LastDecisionPruneRun.ID != "" {
		lastDecisionPruneRun = &LastDecisionPruneRunResponse{
			ID: stats.LastDecisionPruneRun.ID, Before: stats.LastDecisionPruneRun.Before.Format(time.RFC3339),
			Deleted: stats.LastDecisionPruneRun.Deleted, DryRun: stats.LastDecisionPruneRun.DryRun,
			CreatedAt: stats.LastDecisionPruneRun.CreatedAt.Format(time.RFC3339),
		}
	}
	var lastReconciliationRun *ReconciliationRunResponse
	if stats.LastReconciliationRun.ID != "" {
		response := reconciliationRunResponse(stats.LastReconciliationRun)
		lastReconciliationRun = &response
	}
	workerHealth := make([]WorkerHealthResponse, 0, len(stats.WorkerHealth))
	for _, worker := range stats.WorkerHealth {
		item := WorkerHealthResponse{Name: worker.Name, Status: worker.Status, PendingJobs: worker.PendingJobs, RunningJobs: worker.RunningJobs, FailedJobs: worker.FailedJobs}
		if !worker.StartedAt.IsZero() {
			item.StartedAt = worker.StartedAt.Format(time.RFC3339Nano)
		}
		if !worker.LastHeartbeatAt.IsZero() {
			item.LastHeartbeatAt = worker.LastHeartbeatAt.Format(time.RFC3339Nano)
		}
		if !worker.OldestPendingAt.IsZero() {
			item.OldestPendingAt = worker.OldestPendingAt.Format(time.RFC3339Nano)
		}
		if !worker.LastSuccessAt.IsZero() {
			item.LastSuccessAt = worker.LastSuccessAt.Format(time.RFC3339Nano)
		}
		if !worker.LastFailureAt.IsZero() {
			item.LastFailureAt = worker.LastFailureAt.Format(time.RFC3339Nano)
		}
		workerHealth = append(workerHealth, item)
	}

	return StatsResponse{
		Meters:                stats.Meters,
		UsageEvents:           stats.UsageEvents,
		PruneRuns:             stats.PruneRuns,
		LastPruneRun:          lastPruneRun,
		ExportCleanupRuns:     stats.ExportCleanupRuns,
		LastExportCleanupRun:  lastExportCleanupRun,
		ConsumptionDecisions:  stats.ConsumptionDecisions,
		DecisionPruneRuns:     stats.DecisionPruneRuns,
		LastDecisionPruneRun:  lastDecisionPruneRun,
		LastReconciliationRun: lastReconciliationRun,
		ReconciliationHealth:  reconciliationHealthResponse(stats.ReconciliationHealth),
		WorkerHealth:          workerHealth,
		RollupHealth:          rollupHealthResponse(stats.RollupHealth),
	}
}

func rollupHealthResponse(health appsystem.RollupHealth) RollupHealthResponse {
	response := RollupHealthResponse{Status: health.Status, Meters: health.Meters, HealthyMeters: health.HealthyMeters, Issues: health.Issues, Items: make([]RollupMeterHealthResponse, 0, len(health.Items))}
	if !health.FinalizedThrough.IsZero() {
		response.FinalizedThrough = health.FinalizedThrough.Format(time.RFC3339Nano)
	}
	for _, item := range health.Items {
		mapped := RollupMeterHealthResponse{MeterName: item.MeterName, Status: item.Status, ExpectedThrough: item.ExpectedThrough.Format(time.RFC3339Nano), SourceEvents: item.SourceEvents, RollupRows: item.RollupRows, Issue: item.Issue}
		if !item.FinalizedThrough.IsZero() {
			mapped.FinalizedThrough = item.FinalizedThrough.Format(time.RFC3339Nano)
		}
		if !item.LastRunAt.IsZero() {
			mapped.LastRunAt = item.LastRunAt.Format(time.RFC3339Nano)
		}
		response.Items = append(response.Items, mapped)
	}
	return response
}

func reconciliationHealthResponse(health appsystem.ReconciliationHealth) ReconciliationHealthResponse {
	response := ReconciliationHealthResponse{Status: health.Status, PendingNotifications: health.PendingNotifications, DeadLetterNotifications: health.DeadLetterNotifications}
	if !health.NextRunAt.IsZero() {
		response.NextRunAt = health.NextRunAt.Format(time.RFC3339Nano)
	}
	if !health.LockedUntil.IsZero() {
		response.LockedUntil = health.LockedUntil.Format(time.RFC3339Nano)
	}
	if !health.UpdatedAt.IsZero() {
		response.UpdatedAt = health.UpdatedAt.Format(time.RFC3339Nano)
	}
	return response
}
