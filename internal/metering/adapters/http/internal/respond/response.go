package respond

import (
	"encoding/json"
	"errors"
	"net/http"

	"open-spanner/internal/metering/domain"
)

type errorResponse struct {
	Error string `json:"error"`
}

func ServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrConflict):
		Error(w, http.StatusConflict, err.Error())
	default:
		Error(w, http.StatusInternalServerError, "internal server error")
	}
}

func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, errorResponse{Error: message})
}

func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
