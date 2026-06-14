package auth

import (
	"net/http"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
)

func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h.AuthenticateRequest(r); err != nil {
			respond.ServiceError(w, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}
