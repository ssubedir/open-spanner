package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type AuthRepository struct {
	store *Store
}

func NewAuthRepository(store *Store) *AuthRepository {
	return &AuthRepository{store: store}
}

func (r *AuthRepository) CountUsers(ctx context.Context) (int, error) {
	var count int
	if err := r.store.queryRow(ctx, `SELECT COUNT(*) FROM auth_users`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *AuthRepository) SaveUser(ctx context.Context, user appauth.User) (appauth.User, error) {
	_, err := r.store.exec(ctx, `
INSERT INTO auth_users (id, email, password_hash, created_at)
VALUES (?, ?, ?, ?)
`, user.ID, user.Email, user.PasswordHash, formatTime(user.CreatedAt))
	if err != nil {
		if isUniqueConstraint(err) {
			return appauth.User{}, errors.Join(domain.ErrConflict, err)
		}
		return appauth.User{}, err
	}
	return user, nil
}

func (r *AuthRepository) FindUserByID(ctx context.Context, id string) (appauth.User, error) {
	return r.scanUser(r.store.queryRow(ctx, `
SELECT id, email, password_hash, created_at
FROM auth_users
WHERE id = ?
`, id))
}

func (r *AuthRepository) FindUserByEmail(ctx context.Context, email string) (appauth.User, error) {
	return r.scanUser(r.store.queryRow(ctx, `
SELECT id, email, password_hash, created_at
FROM auth_users
WHERE email = ?
`, email))
}

func (r *AuthRepository) SaveSession(ctx context.Context, session appauth.Session) (appauth.Session, error) {
	_, err := r.store.exec(ctx, `
INSERT INTO auth_sessions (id, user_id, token_hash, expires_at, created_at)
VALUES (?, ?, ?, ?, ?)
`, session.ID, session.UserID, session.TokenHash, formatTime(session.ExpiresAt), formatTime(session.CreatedAt))
	if err != nil {
		if isUniqueConstraint(err) {
			return appauth.Session{}, errors.Join(domain.ErrConflict, err)
		}
		return appauth.Session{}, err
	}
	return session, nil
}

func (r *AuthRepository) FindSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) (appauth.Session, error) {
	return scanSession(r.store.queryRow(ctx, `
SELECT id, user_id, token_hash, expires_at, created_at
FROM auth_sessions
WHERE token_hash = ? AND expires_at > ?
`, tokenHash, formatTime(now)))
}

func (r *AuthRepository) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	_, err := r.store.exec(ctx, `
DELETE FROM auth_sessions
WHERE token_hash = ?
`, tokenHash)
	return err
}

func (r *AuthRepository) scanUser(scanner interface {
	Scan(dest ...any) error
}) (appauth.User, error) {
	var user appauth.User
	var createdAt string
	if err := scanner.Scan(&user.ID, &user.Email, &user.PasswordHash, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appauth.User{}, domain.ErrNotFound
		}
		return appauth.User{}, err
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return appauth.User{}, err
	}
	user.CreatedAt = parsedCreatedAt
	return user, nil
}

func scanSession(scanner interface {
	Scan(dest ...any) error
}) (appauth.Session, error) {
	var session appauth.Session
	var expiresAt string
	var createdAt string
	if err := scanner.Scan(&session.ID, &session.UserID, &session.TokenHash, &expiresAt, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appauth.Session{}, domain.ErrNotFound
		}
		return appauth.Session{}, err
	}

	parsedExpiresAt, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return appauth.Session{}, err
	}
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return appauth.Session{}, err
	}
	session.ExpiresAt = parsedExpiresAt
	session.CreatedAt = parsedCreatedAt
	return session, nil
}
