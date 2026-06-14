package respond

import (
	"encoding/json"
	"errors"
	"net/http"

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
	case errors.Is(err, domain.ErrInvalidInput):
		Error(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		Error(w, http.StatusUnauthorized, "unauthorized", err.Error())
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
	Error(w, http.StatusBadRequest, request.Code(err), request.Message(err))
}

func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
