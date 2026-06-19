package subject

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/request"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
	appsubject "github.com/ssubedir/open-spanner/internal/metering/app/subject"
)

type Handler struct {
	service appsubject.Service
}

func NewHandler(service appsubject.Service) *Handler {
	return &Handler{service: service}
}

// List lists subject activity stats.
//
// @Summary List subjects
// @ID listSubjects
// @Tags subjects
// @Produce json
// @Param limit query int false "Page size"
// @Param cursor query string false "Pagination cursor"
// @Success 200 {object} ListResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/subjects [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	subjects, err := h.service.List(r.Context(), appsubject.ListQuery{
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	res := make([]Response, 0, len(subjects.Items))
	for _, subject := range subjects.Items {
		res = append(res, responseFromResult(subject))
	}

	respond.JSON(w, http.StatusOK, ListResponse{Items: res, NextCursor: subjects.NextCursor})
}

// ListEvents lists recent usage events for a subject.
//
// @Summary List subject usage events
// @ID listSubjectUsageEvents
// @Tags subjects
// @Produce json
// @Param subject path string true "Subject"
// @Param limit query int false "Page size"
// @Success 200 {array} EventResponse
// @Failure 400 {object} respond.ErrorResponse
// @Failure 500 {object} respond.ErrorResponse
// @Router /v1/subjects/{subject}/usageevents [get]
func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	limit, err := request.ParseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		respond.ValidationError(w, err)
		return
	}

	events, err := h.service.ListEvents(r.Context(), appsubject.EventListQuery{
		Subject: chi.URLParam(r, "subject"),
		Limit:   limit,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	res := make([]EventResponse, 0, len(events))
	for _, event := range events {
		res = append(res, eventResponseFromResult(event))
	}

	respond.JSON(w, http.StatusOK, res)
}

func responseFromResult(result appsubject.Result) Response {
	return Response{
		Subject:     result.Subject,
		UsageEvents: result.UsageEvents,
		Meters:      result.Meters,
		LastEventAt: result.LastEventAt.Format(time.RFC3339),
	}
}

func eventResponseFromResult(result appsubject.EventResult) EventResponse {
	return EventResponse{
		ID:             result.ID,
		IdempotencyKey: result.IdempotencyKey,
		Subject:        result.Subject,
		Meter:          result.MeterName,
		Quantity:       result.Quantity,
		Timestamp:      result.EventTime.Format(time.RFC3339),
		ReceivedAt:     result.ReceivedAt.Format(time.RFC3339),
		Metadata:       result.Metadata,
	}
}
