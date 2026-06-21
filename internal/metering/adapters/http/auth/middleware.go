package auth

import (
	"context"
	"net/http"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
)

func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := h.authenticateRequestPrincipal(r)
		if err != nil {
			respond.ServiceError(w, err)
			return
		}
		next.ServeHTTP(w, withPrincipal(r, principal))
	})
}

func (h *Handler) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := h.currentPrincipal(r)
		if err != nil {
			respond.ServiceError(w, err)
			return
		}
		next.ServeHTTP(w, withPrincipal(r, principal))
	})
}

func UserFromContext(ctx context.Context) (appauth.UserResult, bool) {
	return appauth.UserFromContext(ctx)
}

func PrincipalFromContext(ctx context.Context) (appauth.Principal, bool) {
	return appauth.PrincipalFromContext(ctx)
}

func withPrincipal(r *http.Request, principal appauth.Principal) *http.Request {
	return r.WithContext(appauth.WithPrincipal(r.Context(), principal))
}
