package usage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/fileexport"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/request"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
	appalert "github.com/ssubedir/open-spanner/internal/metering/app/alert"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type Handler struct {
	service     appusage.Service
	alerts      alertEnqueuer
	exportStore fileexport.Store
}

type alertEnqueuer interface {
	EnqueueForUsageEvents(ctx context.Context, events []appalert.UsageEvent) error
}

func NewHandler(service appusage.Service, exportStoragePaths ...string) *Handler {
	return NewHandlerWithAlerts(service, nil, exportStoragePaths...)
}

func NewHandlerWithAlerts(service appusage.Service, alerts alertEnqueuer, exportStoragePaths ...string) *Handler {
	exportStoragePath := "open-spanner-exports"
	if len(exportStoragePaths) > 0 && strings.TrimSpace(exportStoragePaths[0]) != "" {
		exportStoragePath = exportStoragePaths[0]
	}
	return &Handler{service: service, alerts: alerts, exportStore: fileexport.NewStore(exportStoragePath)}
}

// Create creates a usage event.
//
// @Summary Create usage
// @Description Records one usage event. If idempotency_key matches a previously accepted event, the original event is returned. A duplicate event ID is a conflict.
// @ID createUsage
// @Tags usages
// @Accept json
// @Produce json
// @Param request body CreateRequest true "Usage event"
// @Success 201 {object} Response
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/usages [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}

	eventTime, err := request.OptionalTime("timestamp", req.Timestamp)
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	event, err := h.service.Create(r.Context(), appusage.CreateCommand{
		IdempotencyKey: req.IdempotencyKey,
		Subject:        req.Subject,
		MeterName:      req.Meter,
		Quantity:       req.Quantity,
		EventTime:      eventTime,
		Metadata:       req.Metadata,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	if _, err := h.service.RecordIngestion(r.Context(), appusage.IngestionCommand{
		Kind:     "single",
		Accepted: 1,
	}); err != nil {
		respond.ServiceError(w, err)
		return
	}
	h.enqueueAlerts(r.Context(), []appusage.Result{event})

	respond.JSON(w, http.StatusCreated, responseFromResult(event))
}

// CreateBulk creates usage events in bulk.
//
// @Summary Create usage in bulk
// @Description Records up to 1000 usage events. The Idempotency-Key header replays the original bulk response for the same batch. Per-event idempotency_key values replay existing events as duplicates. Duplicate event IDs are conflicts.
// @ID createUsageBulk
// @Tags usages
// @Accept json
// @Produce json
// @Param Idempotency-Key header string false "Batch idempotency key. Reusing it returns the original bulk response."
// @Param request body []CreateRequest true "Usage events. Maximum 1000 items."
// @Success 201 {object} BulkResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/usages/bulk [post]
func (h *Handler) CreateBulk(w http.ResponseWriter, r *http.Request) {
	var req []CreateRequest
	if err := request.DecodeJSONArray(r.Body, &req, func() int { return len(req) }, appusage.MaxBulkEvents, "bulk usage event"); err != nil {
		respond.ValidationError(w, err)
		return
	}

	commands := make([]appusage.CreateCommand, 0, len(req))
	failures := []appusage.BulkFailureResult{}
	for index, item := range req {
		eventTime, err := request.OptionalTime("timestamp", item.Timestamp)
		if err != nil {
			failures = append(failures, appusage.BulkFailureResult{
				Index:   index,
				Code:    request.Code(err),
				Message: request.Message(err),
			})
			continue
		}

		commands = append(commands, appusage.CreateCommand{
			Index:          index,
			IdempotencyKey: item.IdempotencyKey,
			Subject:        item.Subject,
			MeterName:      item.Meter,
			Quantity:       item.Quantity,
			EventTime:      eventTime,
			Metadata:       item.Metadata,
		})
	}

	result := appusage.BulkResult{Failed: failures}
	if len(commands) > 0 || len(failures) == 0 {
		serviceResult, err := h.service.CreateBulk(r.Context(), r.Header.Get("Idempotency-Key"), commands)
		if err != nil {
			respond.ServiceError(w, err)
			return
		}
		result.Accepted = serviceResult.Accepted
		result.Duplicates = serviceResult.Duplicates
		result.Failed = append(result.Failed, serviceResult.Failed...)
	}

	status := http.StatusCreated
	if len(result.Accepted) == 0 && len(result.Duplicates) == 0 && len(result.Failed) > 0 {
		status = http.StatusBadRequest
	}

	if _, err := h.service.RecordIngestion(r.Context(), appusage.IngestionCommand{
		Kind:       "bulk",
		Accepted:   len(result.Accepted),
		Duplicates: len(result.Duplicates),
		Failed:     len(result.Failed),
	}); err != nil {
		respond.ServiceError(w, err)
		return
	}
	h.enqueueAlerts(r.Context(), result.Accepted)

	respond.JSON(w, status, bulkResponseFromResult(result))
}

// PruneEvents prunes raw usage events using meter retention policy.
//
// @Summary Prune usage events
// @ID pruneUsageEvents
// @Tags usage-events
// @Produce json
// @Param dry_run query bool false "Count prunable events without deleting"
// @Success 200 {object} PruneResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/usageevents/prune [post]
func (h *Handler) PruneEvents(w http.ResponseWriter, r *http.Request) {
	dryRun, err := request.ParseOptionalBool("dry_run", r.URL.Query().Get("dry_run"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	result, err := h.service.PruneEvents(r.Context(), appusage.PruneCommand{DryRun: dryRun})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, pruneResponseFromResult(result))
}

// ListPruneRuns lists usage event prune runs.
//
// @Summary List prune runs
// @ID listPruneRuns
// @Tags usage-events
// @Produce json
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} PruneListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/usageevents/prunes [get]
func (h *Handler) ListPruneRuns(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	runs, err := h.service.ListPruneRuns(r.Context(), appusage.PruneRunListQuery{
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	res := make([]PruneResponse, 0, len(runs.Items))
	for _, run := range runs.Items {
		res = append(res, pruneResponseFromResult(run))
	}

	respond.JSON(w, http.StatusOK, PruneListResponse{Items: res, NextCursor: runs.NextCursor})
}

// ListIngestions lists ingestion summaries.
//
// @Summary List ingestion runs
// @ID listIngestionRuns
// @Tags usages
// @Produce json
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} IngestionListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/usageingestions [get]
func (h *Handler) ListIngestions(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	runs, err := h.service.ListIngestions(r.Context(), appusage.IngestionListQuery{
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	res := make([]IngestionResponse, 0, len(runs.Items))
	for _, run := range runs.Items {
		res = append(res, ingestionResponseFromResult(run))
	}

	respond.JSON(w, http.StatusOK, IngestionListResponse{Items: res, NextCursor: runs.NextCursor})
}

// CreateExportJob queues an async usage export.
//
// @Summary Queue usage export
// @Description Queues a usage bucket export job. Jobs are created as queued and can be listed or inspected by ID.
// @ID createUsageExportJob
// @Tags exports
// @Accept json
// @Produce json
// @Param request body ExportJobCreateRequest true "Usage export job"
// @Success 202 {object} ExportJobResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/exports [post]
func (h *Handler) CreateExportJob(w http.ResponseWriter, r *http.Request) {
	var req ExportJobCreateRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}

	queryJSON, err := exportJobQueryJSON(req.Query)
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	job, err := h.service.CreateExportJob(r.Context(), appusage.ExportJobCreateCommand{
		Kind:      req.Kind,
		Format:    req.Format,
		QueryJSON: queryJSON,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusAccepted, exportJobResponseFromResult(job))
}

// ListExportJobs lists async usage export jobs.
//
// @Summary List usage export jobs
// @ID listUsageExportJobs
// @Tags exports
// @Produce json
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} ExportJobListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/exports [get]
func (h *Handler) ListExportJobs(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	jobs, err := h.service.ListExportJobs(r.Context(), appusage.ExportJobListQuery{
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	res := make([]ExportJobResponse, 0, len(jobs.Items))
	for _, job := range jobs.Items {
		res = append(res, exportJobResponseFromResult(job))
	}

	respond.JSON(w, http.StatusOK, ExportJobListResponse{Items: res, NextCursor: jobs.NextCursor})
}

// GetExportJob gets an async usage export job.
//
// @Summary Get usage export job
// @ID getUsageExportJob
// @Tags exports
// @Produce json
// @Param id path string true "Export job ID"
// @Success 200 {object} ExportJobResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/exports/{id} [get]
func (h *Handler) GetExportJob(w http.ResponseWriter, r *http.Request) {
	job, err := h.service.GetExportJob(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, exportJobResponseFromResult(job))
}

// CancelExportJob cancels a queued or running async usage export job.
//
// @Summary Cancel usage export job
// @ID cancelUsageExportJob
// @Tags exports
// @Produce json
// @Param id path string true "Export job ID"
// @Success 200 {object} ExportJobResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/exports/{id}/cancel [post]
func (h *Handler) CancelExportJob(w http.ResponseWriter, r *http.Request) {
	job, err := h.service.CancelExportJob(r.Context(), appusage.ExportJobCancelCommand{
		ID: chi.URLParam(r, "id"),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, exportJobResponseFromResult(job))
}

// RetryExportJob retries a failed or canceled async usage export job.
//
// @Summary Retry usage export job
// @ID retryUsageExportJob
// @Tags exports
// @Produce json
// @Param id path string true "Export job ID"
// @Success 200 {object} ExportJobResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/exports/{id}/retry [post]
func (h *Handler) RetryExportJob(w http.ResponseWriter, r *http.Request) {
	job, err := h.service.RetryExportJob(r.Context(), appusage.ExportJobRetryCommand{
		ID: chi.URLParam(r, "id"),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, exportJobResponseFromResult(job))
}

// DownloadExportJob downloads a completed async usage export.
//
// @Summary Download usage export job
// @ID downloadUsageExportJob
// @Tags exports
// @Produce text/csv
// @Param id path string true "Export job ID"
// @Success 200 {string} string "CSV"
// @Failure 404 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/exports/{id}/download [get]
func (h *Handler) DownloadExportJob(w http.ResponseWriter, r *http.Request) {
	job, err := h.service.GetExportJob(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	if job.Status != string(domainusage.ExportJobCompleted) || job.ArtifactPath == "" {
		respond.ServiceError(w, errors.Join(domain.ErrConflict, fmt.Errorf("export job is not ready for download")))
		return
	}

	file, info, err := h.exportStore.Open(job.ArtifactPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			respond.ServiceError(w, errors.Join(domain.ErrNotFound, fmt.Errorf("export artifact was not found")))
			return
		}
		respond.ServiceError(w, err)
		return
	}
	defer file.Close()

	filename := fmt.Sprintf("open-spanner-export-%s.csv", job.ID)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	http.ServeContent(w, r, filename, info.ModTime(), file)
}

// List lists bucketed usage.
//
// @Summary List usage buckets
// @ID listUsageBuckets
// @Tags usages
// @Produce json
// @Param subject query string false "Subject"
// @Param meter query string true "Meter name"
// @Param from query string true "RFC3339 start time"
// @Param to query string true "RFC3339 end time"
// @Param bucket_size query string false "Bucket size: hour, day, month"
// @Param group_by query []string false "Subject or metadata keys to group by. Repeat the parameter or use comma-separated values." collectionFormat(multi)
// @Param limit query int false "Result limit"
// @Success 200 {array} ListItemResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/usages [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	listQuery, ok := h.listQuery(w, r)
	if !ok {
		return
	}

	buckets, err := h.service.List(r.Context(), listQuery)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, listItemResponses(buckets))
}

// Search searches bucketed usage with an advanced filter.
//
// @Summary Search usage buckets
// @ID searchUsageBuckets
// @Tags usages
// @Accept json
// @Produce json
// @Param request body SearchRequest true "Usage search"
// @Success 200 {array} ListItemResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/usages/search [post]
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}

	listQuery, err := searchListQuery(req)
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	buckets, err := h.service.List(r.Context(), listQuery)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, listItemResponses(buckets))
}

// SearchBreakdown searches top aggregated usage breakdown values.
//
// @Summary Search usage breakdown
// @ID searchUsageBreakdown
// @Tags usages
// @Accept json
// @Produce json
// @Param request body BreakdownRequest true "Usage breakdown search"
// @Success 200 {object} BreakdownListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/usages/breakdowns/search [post]
func (h *Handler) SearchBreakdown(w http.ResponseWriter, r *http.Request) {
	var req BreakdownRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}

	from, err := request.RequiredTime("from", req.From)
	if err != nil {
		respond.ValidationError(w, err)
		return
	}
	to, err := request.RequiredTime("to", req.To)
	if err != nil {
		respond.ValidationError(w, err)
		return
	}
	filter, err := filterFromRequest(req.Filter)
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	breakdown, err := h.service.ListBreakdown(r.Context(), appusage.BreakdownListQuery{
		Subject:   req.Subject,
		MeterName: req.Meter,
		Field:     req.Field,
		From:      from,
		To:        to,
		Limit:     req.Limit,
		Filter:    filter,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	res := make([]BreakdownResponse, 0, len(breakdown.Items))
	for _, item := range breakdown.Items {
		res = append(res, BreakdownResponse{
			Field:       item.Field,
			Value:       item.Value,
			Quantity:    item.Quantity,
			UsageEvents: item.UsageEvents,
			Aggregation: item.Aggregation,
			Unit:        item.Unit,
		})
	}

	respond.JSON(w, http.StatusOK, BreakdownListResponse{Items: res})
}

// ListDimensionValues lists discovered values for a meter metadata dimension.
//
// @Summary List usage dimension values
// @ID listUsageDimensionValues
// @Tags usages
// @Produce json
// @Param meter query string true "Meter name"
// @Param field query string true "Metadata dimension field"
// @Param subject query string false "Subject"
// @Param from query string false "RFC3339 start time"
// @Param to query string false "RFC3339 end time"
// @Param limit query int false "Result limit"
// @Success 200 {object} DimensionValueListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/usages/dimensions [get]
func (h *Handler) ListDimensionValues(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit, err := request.ParseLimit(query.Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}
	from, err := request.OptionalTime("from", query.Get("from"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}
	to, err := request.OptionalTime("to", query.Get("to"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	values, err := h.service.ListDimensionValues(r.Context(), appusage.DimensionValueListQuery{
		MeterName: query.Get("meter"),
		Field:     query.Get("field"),
		Subject:   query.Get("subject"),
		From:      from,
		To:        to,
		Limit:     limit,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	res := make([]DimensionValueResponse, 0, len(values.Items))
	for _, item := range values.Items {
		res = append(res, DimensionValueResponse{
			Field:       item.Field,
			Value:       item.Value,
			UsageEvents: item.UsageEvents,
		})
	}

	respond.JSON(w, http.StatusOK, DimensionValueListResponse{Items: res})
}

// ListEvents lists raw usage events.
//
// @Summary List usage events
// @ID listUsageEvents
// @Tags usage-events
// @Produce json
// @Param subject query string false "Subject"
// @Param meter query string false "Meter name"
// @Param from query string false "RFC3339 start time"
// @Param to query string false "RFC3339 end time"
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} EventListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/usageevents [get]
func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	listQuery, ok := h.eventListQuery(w, r)
	if !ok {
		return
	}

	page, err := h.service.ListEvents(r.Context(), listQuery)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	items := make([]Response, 0, len(page.Items))
	for _, event := range page.Items {
		items = append(items, responseFromResult(event))
	}

	respond.JSON(w, http.StatusOK, EventListResponse{
		Items:      items,
		NextCursor: page.NextCursor,
	})
}

// SearchEvents searches raw usage events with an advanced filter.
//
// @Summary Search usage events
// @ID searchUsageEvents
// @Tags usage-events
// @Accept json
// @Produce json
// @Param request body EventSearchRequest true "Usage event search"
// @Success 200 {object} EventListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/usageevents/search [post]
func (h *Handler) SearchEvents(w http.ResponseWriter, r *http.Request) {
	var req EventSearchRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}

	listQuery, err := searchEventListQuery(req)
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	page, err := h.service.ListEvents(r.Context(), listQuery)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	items := make([]Response, 0, len(page.Items))
	for _, event := range page.Items {
		items = append(items, responseFromResult(event))
	}

	respond.JSON(w, http.StatusOK, EventListResponse{
		Items:      items,
		NextCursor: page.NextCursor,
	})
}

// Export exports bucketed usage as CSV.
//
// @Summary Export usage buckets
// @ID exportUsageBuckets
// @Tags usages
// @Produce text/csv
// @Param subject query string false "Subject"
// @Param meter query string true "Meter name"
// @Param from query string true "RFC3339 start time"
// @Param to query string true "RFC3339 end time"
// @Param bucket_size query string false "Bucket size: hour, day, month"
// @Param group_by query []string false "Subject or metadata keys to group by. Repeat the parameter or use comma-separated values." collectionFormat(multi)
// @Param limit query int false "Result limit. Defaults to 100 and cannot exceed 1000."
// @Success 200 {string} string "CSV"
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/usages/export [get]
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	listQuery, ok := h.listQuery(w, r)
	if !ok {
		return
	}
	if err := validateDirectExportLimit(listQuery.Limit); err != nil {
		respond.ValidationError(w, err)
		return
	}

	buckets, err := h.service.List(r.Context(), listQuery)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	writeBucketCSV(w, listQuery.GroupBy, buckets)
}

// ExportSearch exports bucketed usage matching an advanced filter as CSV.
//
// @Summary Export filtered usage buckets
// @ID exportFilteredUsageBuckets
// @Tags usages
// @Accept json
// @Produce text/csv
// @Param request body SearchRequest true "Usage export search"
// @Success 200 {string} string "CSV"
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/usages/export [post]
func (h *Handler) ExportSearch(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}

	listQuery, err := searchListQuery(req)
	if err != nil {
		respond.ValidationError(w, err)
		return
	}
	if err := validateDirectExportLimit(listQuery.Limit); err != nil {
		respond.ValidationError(w, err)
		return
	}

	buckets, err := h.service.List(r.Context(), listQuery)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	writeBucketCSV(w, listQuery.GroupBy, buckets)
}

func writeBucketCSV(w http.ResponseWriter, groupBy []string, buckets []appusage.ListItemResult) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="usage-buckets.csv"`)
	w.WriteHeader(http.StatusOK)

	_ = appusage.WriteBucketCSV(w, groupBy, buckets)
}

// ExportEvents exports raw usage events as CSV.
//
// @Summary Export usage events
// @ID exportUsageEvents
// @Tags usage-events
// @Produce text/csv
// @Param subject query string false "Subject"
// @Param meter query string false "Meter name"
// @Param from query string false "RFC3339 start time"
// @Param to query string false "RFC3339 end time"
// @Param limit query int false "Result limit. Defaults to 100 and cannot exceed 1000."
// @Success 200 {string} string "CSV"
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/usageevents/export [get]
func (h *Handler) ExportEvents(w http.ResponseWriter, r *http.Request) {
	listQuery, ok := h.eventListQuery(w, r)
	if !ok {
		return
	}
	listQuery.Cursor = ""
	if err := validateDirectExportLimit(listQuery.Limit); err != nil {
		respond.ValidationError(w, err)
		return
	}

	page, err := h.service.ListEvents(r.Context(), listQuery)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	writeEventCSV(w, page.Items)
}

// ExportEventsSearch exports raw usage events matching an advanced filter as CSV.
//
// @Summary Export filtered usage events
// @ID exportFilteredUsageEvents
// @Tags usage-events
// @Accept json
// @Produce text/csv
// @Param request body EventSearchRequest true "Usage event export search"
// @Success 200 {string} string "CSV"
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/usageevents/export [post]
func (h *Handler) ExportEventsSearch(w http.ResponseWriter, r *http.Request) {
	var req EventSearchRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}

	listQuery, err := searchEventListQuery(req)
	if err != nil {
		respond.ValidationError(w, err)
		return
	}
	listQuery.Cursor = ""
	if err := validateDirectExportLimit(listQuery.Limit); err != nil {
		respond.ValidationError(w, err)
		return
	}

	page, err := h.service.ListEvents(r.Context(), listQuery)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	writeEventCSV(w, page.Items)
}

func writeEventCSV(w http.ResponseWriter, events []appusage.Result) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="usage-events.csv"`)
	w.WriteHeader(http.StatusOK)

	_ = appusage.WriteEventCSV(w, events)
}

func (h *Handler) enqueueAlerts(ctx context.Context, events []appusage.Result) {
	if h.alerts == nil || len(events) == 0 {
		return
	}
	alertEvents := make([]appalert.UsageEvent, 0, len(events))
	for _, event := range events {
		alertEvents = append(alertEvents, appalert.UsageEvent{
			Subject:  event.Subject,
			Meter:    event.MeterName,
			Metadata: event.Metadata,
		})
	}
	if err := h.alerts.EnqueueForUsageEvents(ctx, alertEvents); err != nil {
		log.Printf("alert enqueue failed: %v", err)
	}
}

func validateDirectExportLimit(limit int) error {
	return request.ValidateOptionalLimit(limit, domainusage.MaxLimit)
}

func searchListQuery(req SearchRequest) (appusage.ListQuery, error) {
	from, err := request.RequiredTime("from", req.From)
	if err != nil {
		return appusage.ListQuery{}, err
	}
	to, err := request.RequiredTime("to", req.To)
	if err != nil {
		return appusage.ListQuery{}, err
	}
	filter, err := filterFromRequest(req.Filter)
	if err != nil {
		return appusage.ListQuery{}, err
	}

	return appusage.ListQuery{
		Subject:    req.Subject,
		MeterName:  req.Meter,
		From:       from,
		To:         to,
		BucketSize: domainusage.BucketSize(req.BucketSize),
		GroupBy:    req.GroupBy.Fields(),
		Limit:      req.Limit,
		Filter:     filter,
	}, nil
}

func exportJobQueryJSON(query *SearchRequest) (string, error) {
	if query == nil {
		return "", request.NewValidationError("invalid_query", "query is required")
	}
	if _, err := searchListQuery(*query); err != nil {
		return "", err
	}

	payload, err := json.Marshal(query)
	return string(payload), err
}

func searchEventListQuery(req EventSearchRequest) (appusage.EventListQuery, error) {
	from, err := request.OptionalTime("from", req.From)
	if err != nil {
		return appusage.EventListQuery{}, err
	}
	to, err := request.OptionalTime("to", req.To)
	if err != nil {
		return appusage.EventListQuery{}, err
	}
	filter, err := filterFromRequest(req.Filter)
	if err != nil {
		return appusage.EventListQuery{}, err
	}

	return appusage.EventListQuery{
		Subject:   req.Subject,
		MeterName: req.Meter,
		From:      from,
		To:        to,
		Limit:     req.Limit,
		Cursor:    req.Cursor,
		Filter:    filter,
	}, nil
}

func (h *Handler) eventListQuery(w http.ResponseWriter, r *http.Request) (appusage.EventListQuery, bool) {
	query := r.URL.Query()
	limit, err := request.ParseLimit(query.Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return appusage.EventListQuery{}, false
	}

	from, err := request.OptionalTime("from", query.Get("from"))
	if err != nil {
		respond.ValidationError(w, err)
		return appusage.EventListQuery{}, false
	}
	to, err := request.OptionalTime("to", query.Get("to"))
	if err != nil {
		respond.ValidationError(w, err)
		return appusage.EventListQuery{}, false
	}

	return appusage.EventListQuery{
		Subject:   query.Get("subject"),
		MeterName: query.Get("meter"),
		From:      from,
		To:        to,
		Limit:     limit,
		Cursor:    query.Get("cursor"),
	}, true
}

func (h *Handler) listQuery(w http.ResponseWriter, r *http.Request) (appusage.ListQuery, bool) {
	query := r.URL.Query()
	limit, err := request.ParseLimit(query.Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return appusage.ListQuery{}, false
	}

	from, err := request.RequiredTime("from", query.Get("from"))
	if err != nil {
		respond.ValidationError(w, err)
		return appusage.ListQuery{}, false
	}
	to, err := request.RequiredTime("to", query.Get("to"))
	if err != nil {
		respond.ValidationError(w, err)
		return appusage.ListQuery{}, false
	}

	return appusage.ListQuery{
		Subject:    query.Get("subject"),
		MeterName:  query.Get("meter"),
		From:       from,
		To:         to,
		BucketSize: domainusage.BucketSize(query.Get("bucket_size")),
		Metadata:   metadataFilters(query),
		GroupBy:    domainusage.SplitGroupByValues(query["group_by"]),
		Limit:      limit,
	}, true
}

func responseFromResult(event appusage.Result) Response {
	return Response{
		ID:             event.ID,
		IdempotencyKey: event.IdempotencyKey,
		Subject:        event.Subject,
		Meter:          event.MeterName,
		Quantity:       event.Quantity,
		Timestamp:      event.EventTime.Format(time.RFC3339),
		ReceivedAt:     event.ReceivedAt.Format(time.RFC3339),
		Metadata:       event.Metadata,
	}
}

func listItemResponses(buckets []appusage.ListItemResult) []ListItemResponse {
	res := make([]ListItemResponse, 0, len(buckets))
	for _, bucket := range buckets {
		res = append(res, ListItemResponse{
			Subject:     bucket.Subject,
			Meter:       bucket.MeterName,
			BucketSize:  bucket.BucketSize,
			BucketStart: bucket.BucketStart.Format(time.RFC3339),
			Aggregation: bucket.Aggregation,
			Unit:        bucket.Unit,
			Quantity:    bucket.Quantity,
			Group:       bucket.Group,
		})
	}
	return res
}

func bulkResponseFromResult(result appusage.BulkResult) BulkResponse {
	accepted := make([]Response, 0, len(result.Accepted))
	for _, event := range result.Accepted {
		accepted = append(accepted, responseFromResult(event))
	}

	duplicates := make([]Response, 0, len(result.Duplicates))
	for _, event := range result.Duplicates {
		duplicates = append(duplicates, responseFromResult(event))
	}

	return BulkResponse{
		AcceptedCount:  len(accepted),
		DuplicateCount: len(duplicates),
		FailedCount:    len(result.Failed),
		Accepted:       accepted,
		Duplicates:     duplicates,
		Failed:         bulkFailureResponses(result.Failed),
	}
}

func bulkFailureResponses(failures []appusage.BulkFailureResult) []BulkFailureResponse {
	sort.Slice(failures, func(i, j int) bool {
		return failures[i].Index < failures[j].Index
	})

	res := make([]BulkFailureResponse, 0, len(failures))
	for _, failure := range failures {
		res = append(res, BulkFailureResponse{
			Index:   failure.Index,
			Code:    failure.Code,
			Message: failure.Message,
		})
	}
	return res
}

func pruneResponseFromResult(result appusage.PruneResult) PruneResponse {
	meters := make([]PruneMeterResponse, 0, len(result.Meters))
	for _, meter := range result.Meters {
		meters = append(meters, PruneMeterResponse{
			Meter:   meter.MeterName,
			Before:  meter.Before.Format(time.RFC3339),
			Deleted: meter.Deleted,
		})
	}

	return PruneResponse{
		ID:        result.ID,
		Deleted:   result.Deleted,
		DryRun:    result.DryRun,
		Meters:    meters,
		CreatedAt: result.CreatedAt.Format(time.RFC3339),
	}
}

func ingestionResponseFromResult(result appusage.IngestionResult) IngestionResponse {
	return IngestionResponse{
		ID:         result.ID,
		Kind:       result.Kind,
		Accepted:   result.Accepted,
		Duplicates: result.Duplicates,
		Failed:     result.Failed,
		CreatedAt:  result.CreatedAt.Format(time.RFC3339),
	}
}

func exportJobResponseFromResult(result appusage.ExportJobResult) ExportJobResponse {
	var query SearchRequest
	_ = json.Unmarshal([]byte(result.QueryJSON), &query)

	res := ExportJobResponse{
		ID:           result.ID,
		Kind:         result.Kind,
		Status:       result.Status,
		Format:       result.Format,
		Query:        query,
		Error:        result.ErrorMessage,
		ArtifactSize: result.ArtifactSize,
		CreatedAt:    result.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    result.UpdatedAt.Format(time.RFC3339),
	}
	if result.Status == string(domainusage.ExportJobCompleted) && result.ArtifactPath != "" {
		res.DownloadURL = "/v1/exports/" + result.ID + "/download"
	}
	if !result.CompletedAt.IsZero() {
		res.CompletedAt = result.CompletedAt.Format(time.RFC3339)
	}
	return res
}

func metadataFilters(query map[string][]string) map[string]string {
	const prefix = "metadata."

	filters := map[string]string{}
	for key, values := range query {
		if !strings.HasPrefix(key, prefix) || len(values) == 0 {
			continue
		}
		filters[strings.TrimPrefix(key, prefix)] = values[0]
	}

	return filters
}
