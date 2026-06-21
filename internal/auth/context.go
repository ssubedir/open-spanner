package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

const DefaultWorkspaceID = "default"

type principalContextKey struct{}
type workspaceContextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	ctx = context.WithValue(ctx, principalContextKey{}, principal)
	if strings.TrimSpace(principal.WorkspaceID) != "" {
		ctx = WithWorkspaceID(ctx, principal.WorkspaceID)
	}
	return ctx
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}

func UserFromContext(ctx context.Context) (UserResult, bool) {
	if principal, ok := PrincipalFromContext(ctx); ok {
		return principal.User, true
	}
	return UserResult{}, false
}

func WithWorkspaceID(ctx context.Context, workspaceID string) context.Context {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return ctx
	}
	return context.WithValue(ctx, workspaceContextKey{}, workspaceID)
}

func WorkspaceIDFromContext(ctx context.Context) (string, bool) {
	if workspaceID, ok := ctx.Value(workspaceContextKey{}).(string); ok && strings.TrimSpace(workspaceID) != "" {
		return workspaceID, true
	}
	if principal, ok := PrincipalFromContext(ctx); ok && strings.TrimSpace(principal.WorkspaceID) != "" {
		return principal.WorkspaceID, true
	}
	return "", false
}

func RequireWorkspaceID(ctx context.Context) (string, error) {
	if workspaceID, ok := WorkspaceIDFromContext(ctx); ok {
		return workspaceID, nil
	}
	return "", errors.Join(domain.ErrUnauthorized, errors.New("workspace is required"))
}
