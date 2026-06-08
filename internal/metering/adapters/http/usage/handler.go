package usage

import (
	"encoding/json"
	"net/http"
	"time"

	"open-spanner/internal/metering/adapters/http/internal/respond"
	"open-spanner/internal/metering/adapters/http/internal/timeparse"
	appusage "open-spanner/internal/metering/app/usage"
	domainusage "open-spanner/internal/metering/domain/usage"
)

type Handler struct {
	service appusage.Service
}

func NewHandler(service appusage.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	eventTime, err := timeparse.Optional(req.Timestamp)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "timestamp must be RFC3339")
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

	respond.JSON(w, http.StatusCreated, response{
		ID:             event.ID,
		IdempotencyKey: event.IdempotencyKey,
		Subject:        event.Subject,
		Meter:          event.MeterName,
		Quantity:       event.Quantity,
		Timestamp:      event.EventTime.Format(time.RFC3339),
		ReceivedAt:     event.ReceivedAt.Format(time.RFC3339),
		Metadata:       event.Metadata,
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	from, err := timeparse.Required(query.Get("from"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "from must be RFC3339")
		return
	}
	to, err := timeparse.Required(query.Get("to"))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "to must be RFC3339")
		return
	}

	buckets, err := h.service.List(r.Context(), appusage.ListQuery{
		Subject:    query.Get("subject"),
		MeterName:  query.Get("meter"),
		From:       from,
		To:         to,
		BucketSize: domainusage.BucketSize(query.Get("bucket_size")),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	res := make([]listItemResponse, 0, len(buckets))
	for _, bucket := range buckets {
		res = append(res, listItemResponse{
			Subject:     bucket.Subject,
			Meter:       bucket.MeterName,
			BucketSize:  bucket.BucketSize,
			BucketStart: bucket.BucketStart.Format(time.RFC3339),
			Quantity:    bucket.Quantity,
		})
	}

	respond.JSON(w, http.StatusOK, res)
}
