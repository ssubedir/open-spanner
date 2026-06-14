package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

func TestAuthRepositoryUserAndSessionFlow(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	repo := NewAuthRepository(store)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	count, err := repo.CountUsers(ctx)
	if err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Fatalf("user count = %d, want 0", count)
	}

	user := appauth.User{
		ID:           "user-1",
		Email:        "admin@example.com",
		PasswordHash: "hashed-password",
		CreatedAt:    now,
	}
	if _, err := repo.SaveUser(ctx, user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	count, err = repo.CountUsers(ctx)
	if err != nil {
		t.Fatalf("count users after save: %v", err)
	}
	if count != 1 {
		t.Fatalf("user count = %d, want 1", count)
	}

	found, err := repo.FindUserByEmail(ctx, "admin@example.com")
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if found.ID != user.ID || found.PasswordHash != user.PasswordHash {
		t.Fatalf("found user = %#v, want %#v", found, user)
	}

	session := appauth.Session{
		ID:        "session-1",
		UserID:    user.ID,
		TokenHash: appauth.HashToken("session-token"),
		Kind:      appauth.TokenKindAccess,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}
	if _, err := repo.SaveSession(ctx, session); err != nil {
		t.Fatalf("save session: %v", err)
	}

	active, err := repo.FindSessionByTokenHash(ctx, session.TokenHash, appauth.TokenKindAccess, now)
	if err != nil {
		t.Fatalf("find active session: %v", err)
	}
	if active.ID != session.ID || active.UserID != user.ID || active.Kind != appauth.TokenKindAccess {
		t.Fatalf("active session = %#v, want %#v", active, session)
	}

	_, err = repo.FindSessionByTokenHash(ctx, session.TokenHash, appauth.TokenKindRefresh, now)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("wrong kind session error = %v, want ErrNotFound", err)
	}

	_, err = repo.FindSessionByTokenHash(ctx, session.TokenHash, appauth.TokenKindAccess, now.Add(2*time.Hour))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expired session error = %v, want ErrNotFound", err)
	}
}

func TestAuthRepositoryDuplicateEmailReturnsConflict(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	repo := NewAuthRepository(store)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	first := appauth.User{ID: "user-1", Email: "admin@example.com", PasswordHash: "hash-1", CreatedAt: now}
	second := appauth.User{ID: "user-2", Email: "admin@example.com", PasswordHash: "hash-2", CreatedAt: now}
	if _, err := repo.SaveUser(ctx, first); err != nil {
		t.Fatalf("save first user: %v", err)
	}

	_, err := repo.SaveUser(ctx, second)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate user error = %v, want ErrConflict", err)
	}
}
