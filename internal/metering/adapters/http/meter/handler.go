package meter

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/request"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
	appmeter "github.com/ssubedir/open-spanner/internal/metering/app/meter"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
)

type Handler struct {
	service appmeter.Service
}

func NewHandler(service appmeter.Service) *Handler {
	return &Handler{service: service}
}

// Create creates a meter.
//
// @Summary Create meter
// @ID createMeter
// @Tags meters
// @Accept json
// @Produce json
// @Param request body CreateRequest true "Meter"
// @Success 201 {object} Response
// @Failure 400 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/meters [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}

	meter, err := h.service.Create(r.Context(), appmeter.CreateCommand{
		Name:               req.Name,
		Description:        req.Description,
		Unit:               req.Unit,
		Aggregation:        domainmeter.Aggregation(req.Aggregation),
		MetadataSchema:     metadataSchemaFromRequest(req.MetadataSchema),
		EventRetentionDays: req.EventRetentionDays,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, responseFromResult(meter))
}

// List lists meters.
//
// @Summary List meters
// @ID listMeters
// @Tags meters
// @Produce json
// @Param name query string false "Filter by meter name"
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} ListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/meters [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	meters, err := h.service.List(r.Context(), appmeter.ListQuery{
		Name:   r.URL.Query().Get("name"),
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	res := make([]Response, 0, len(meters.Items))
	for _, meter := range meters.Items {
		res = append(res, responseFromResult(meter))
	}

	respond.JSON(w, http.StatusOK, ListResponse{Items: res, NextCursor: meters.NextCursor})
}

// ListStats lists meter activity stats.
//
// @Summary List meter stats
// @ID listMeterStats
// @Tags meters
// @Produce json
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} StatsListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/meters/stats [get]
func (h *Handler) ListStats(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	stats, err := h.service.ListStats(r.Context(), appmeter.StatsListQuery{
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	res := make([]StatsResponse, 0, len(stats.Items))
	for _, stat := range stats.Items {
		res = append(res, statsResponseFromResult(stat))
	}

	respond.JSON(w, http.StatusOK, StatsListResponse{Items: res, NextCursor: stats.NextCursor})
}

// Get gets a meter by id.
//
// @Summary Get meter
// @ID getMeter
// @Tags meters
// @Produce json
// @Param id path string true "Meter ID"
// @Success 200 {object} Response
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/meters/{id} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	meter, err := h.service.Get(r.Context(), appmeter.GetQuery{
		ID: chi.URLParam(r, "id"),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, responseFromResult(meter))
}

// Update updates a meter.
//
// @Summary Update meter
// @ID updateMeter
// @Tags meters
// @Accept json
// @Produce json
// @Param id path string true "Meter ID"
// @Param request body UpdateRequest true "Meter update"
// @Success 200 {object} Response
// @Failure 400 {object} respond.ErrorResponse
// @Failure 404 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/meters/{id} [put]
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var req UpdateRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}

	meter, err := h.service.Update(r.Context(), appmeter.UpdateCommand{
		ID:          chi.URLParam(r, "id"),
		Description: req.Description,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, responseFromResult(meter))
}

// Delete deletes a meter.
//
// @Summary Delete meter
// @ID deleteMeter
// @Tags meters
// @Produce json
// @Param id path string true "Meter ID"
// @Success 204
// @Failure 404 {object} respond.ErrorResponse
// @Failure 409 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/meters/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), appmeter.DeleteCommand{ID: chi.URLParam(r, "id")}); err != nil {
		respond.ServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func statsResponseFromResult(stat appmeter.StatsResult) StatsResponse {
	res := StatsResponse{
		Meter:              stat.MeterName,
		UsageEvents:        stat.UsageEvents,
		EventRetentionDays: stat.EventRetentionDays,
	}
	if !stat.LastEventAt.IsZero() {
		res.LastEventAt = stat.LastEventAt.Format(time.RFC3339)
	}
	return res
}

func responseFromResult(meter appmeter.Result) Response {
	return Response{
		ID:                 meter.ID,
		Name:               meter.Name,
		Description:        meter.Description,
		Unit:               meter.Unit,
		Aggregation:        meter.Aggregation,
		MetadataSchema:     meter.MetadataSchema,
		EventRetentionDays: meter.EventRetentionDays,
		CreatedAt:          meter.CreatedAt.Format(time.RFC3339),
	}
}

func metadataSchemaFromRequest(input map[string]string) map[string]domainmeter.MetadataType {
	schema := map[string]domainmeter.MetadataType{}
	for key, value := range input {
		schema[key] = domainmeter.MetadataType(value)
	}
	return schema
}
