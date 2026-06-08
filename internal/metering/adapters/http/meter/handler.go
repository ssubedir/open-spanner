package meter

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"open-spanner/internal/metering/adapters/http/internal/respond"
	appmeter "open-spanner/internal/metering/app/meter"
	domainmeter "open-spanner/internal/metering/domain/meter"
)

type Handler struct {
	service appmeter.Service
}

func NewHandler(service appmeter.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	meter, err := h.service.Create(r.Context(), appmeter.CreateCommand{
		Name:        req.Name,
		Description: req.Description,
		Unit:        req.Unit,
		Aggregation: domainmeter.Aggregation(req.Aggregation),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, responseFromResult(meter))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	meters, err := h.service.List(r.Context(), appmeter.ListQuery{
		Name: r.URL.Query().Get("name"),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	res := make([]response, 0, len(meters))
	for _, meter := range meters {
		res = append(res, responseFromResult(meter))
	}

	respond.JSON(w, http.StatusOK, res)
}

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

func responseFromResult(meter appmeter.Result) response {
	return response{
		ID:          meter.ID,
		Name:        meter.Name,
		Description: meter.Description,
		Unit:        meter.Unit,
		Aggregation: meter.Aggregation,
		CreatedAt:   meter.CreatedAt.Format(time.RFC3339),
	}
}
