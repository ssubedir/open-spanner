package auth

import (
	"context"
	"net/http"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
)

type userContextKey struct{}

func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := h.authenticateRequestUser(r)
		if err != nil {
			respond.ServiceError(w, err)
			return
		}
		next.ServeHTTP(w, withUser(r, user))
	})
}

func (h *Handler) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := h.currentUser(r)
		if err != nil {
			respond.ServiceError(w, err)
			return
		}
		next.ServeHTTP(w, withUser(r, user))
	})
}

func UserFromContext(ctx context.Context) (appauth.UserResult, bool) {
	user, ok := ctx.Value(userContextKey{}).(appauth.UserResult)
	return user, ok
}

func withUser(r *http.Request, user appauth.UserResult) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey{}, user)
	return r.WithContext(ctx)
}
