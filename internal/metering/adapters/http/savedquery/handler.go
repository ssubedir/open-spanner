package savedquery

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	httpauth "github.com/ssubedir/open-spanner/internal/metering/adapters/http/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/request"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
	appsavedquery "github.com/ssubedir/open-spanner/internal/metering/app/savedquery"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type Handler struct {
	service appsavedquery.Service
}

func NewHandler(service appsavedquery.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := httpauth.UserFromContext(r.Context())
	if !ok {
		respond.ServiceError(w, domain.ErrUnauthorized)
		return
	}

	queries, err := h.service.List(r.Context(), appsavedquery.ListQuery{UserID: user.ID})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	res := make([]Response, 0, len(queries.Items))
	for _, item := range queries.Items {
		res = append(res, responseFromResult(item))
	}
	respond.JSON(w, http.StatusOK, ListResponse{Items: res})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := httpauth.UserFromContext(r.Context())
	if !ok {
		respond.ServiceError(w, domain.ErrUnauthorized)
		return
	}

	var req SaveRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}

	saved, err := h.service.Create(r.Context(), appsavedquery.SaveCommand{
		UserID:     user.ID,
		Name:       req.Name,
		Query:      req.Query,
		GroupBy:    req.GroupBy,
		BucketSize: req.BucketSize,
		Limit:      req.Limit,
		Pinned:     boolValue(req.Pinned),
		Position:   intValue(req.Position),
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, responseFromResult(saved))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	user, ok := httpauth.UserFromContext(r.Context())
	if !ok {
		respond.ServiceError(w, domain.ErrUnauthorized)
		return
	}

	var req SaveRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}

	saved, err := h.service.Update(r.Context(), appsavedquery.UpdateCommand{
		ID:         chi.URLParam(r, "id"),
		UserID:     user.ID,
		Name:       req.Name,
		Query:      req.Query,
		GroupBy:    req.GroupBy,
		BucketSize: req.BucketSize,
		Limit:      req.Limit,
		Pinned:     req.Pinned,
		Position:   req.Position,
	})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, responseFromResult(saved))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	user, ok := httpauth.UserFromContext(r.Context())
	if !ok {
		respond.ServiceError(w, domain.ErrUnauthorized)
		return
	}

	if err := h.service.Delete(r.Context(), appsavedquery.DeleteCommand{
		ID:     chi.URLParam(r, "id"),
		UserID: user.ID,
	}); err != nil {
		respond.ServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func responseFromResult(result appsavedquery.Result) Response {
	return Response{
		ID:         result.ID,
		Name:       result.Name,
		Query:      result.Query,
		GroupBy:    result.GroupBy,
		BucketSize: result.BucketSize,
		Limit:      result.Limit,
		Pinned:     result.Pinned,
		Position:   result.Position,
		CreatedAt:  result.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  result.UpdatedAt.Format(time.RFC3339),
	}
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
