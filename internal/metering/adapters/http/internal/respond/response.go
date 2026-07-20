package respond

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/request"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

// ErrorResponse is the standard API error envelope.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody is the standard API error body.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func ServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrRateLimited):
		retryAfter := time.Second
		var value interface{ RetryAfter() time.Duration }
		if errors.As(err, &value) && value.RetryAfter() > 0 {
			retryAfter = value.RetryAfter()
		}
		seconds := int((retryAfter + time.Second - 1) / time.Second)
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
		Error(w, http.StatusTooManyRequests, "rate_limited", err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		Error(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		Error(w, http.StatusUnauthorized, "unauthorized", err.Error())
	case errors.Is(err, domain.ErrForbidden):
		Error(w, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, domain.ErrNotFound):
		Error(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, domain.ErrConflict):
		Error(w, http.StatusConflict, "conflict", err.Error())
	default:
		Error(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func Error(w http.ResponseWriter, status int, code string, message string) {
	JSON(w, status, ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: message,
		},
	})
}

func ValidationError(w http.ResponseWriter, err error) {
	if request.Code(err) == "request_too_large" {
		Error(w, http.StatusRequestEntityTooLarge, request.Code(err), request.Message(err))
		return
	}
	Error(w, http.StatusBadRequest, request.Code(err), request.Message(err))
}

func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
