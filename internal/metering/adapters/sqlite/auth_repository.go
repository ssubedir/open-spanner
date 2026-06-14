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
INSERT INTO auth_sessions (id, user_id, token_hash, kind, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?)
`, session.ID, session.UserID, session.TokenHash, session.Kind, formatTime(session.ExpiresAt), formatTime(session.CreatedAt))
	if err != nil {
		if isUniqueConstraint(err) {
			return appauth.Session{}, errors.Join(domain.ErrConflict, err)
		}
		return appauth.Session{}, err
	}
	return session, nil
}

func (r *AuthRepository) FindSessionByTokenHash(ctx context.Context, tokenHash string, kind string, now time.Time) (appauth.Session, error) {
	return scanSession(r.store.queryRow(ctx, `
SELECT id, user_id, token_hash, kind, expires_at, created_at
FROM auth_sessions
WHERE token_hash = ? AND kind = ? AND expires_at > ?
`, tokenHash, kind, formatTime(now)))
}

func (r *AuthRepository) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	_, err := r.store.exec(ctx, `
DELETE FROM auth_sessions
WHERE token_hash = ?
`, tokenHash)
	return err
}

func (r *AuthRepository) SaveAPIKey(ctx context.Context, key appauth.APIKey) (appauth.APIKey, error) {
	_, err := r.store.exec(ctx, `
INSERT INTO auth_api_keys (id, user_id, name, token_hash, prefix, created_at, last_used_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, key.ID, key.UserID, key.Name, key.TokenHash, key.Prefix, formatTime(key.CreatedAt), formatOptionalTime(key.LastUsedAt))
	if err != nil {
		if isUniqueConstraint(err) {
			return appauth.APIKey{}, errors.Join(domain.ErrConflict, err)
		}
		return appauth.APIKey{}, err
	}
	return key, nil
}

func (r *AuthRepository) ListAPIKeys(ctx context.Context, userID string) ([]appauth.APIKey, error) {
	rows, err := r.store.query(ctx, `
SELECT id, user_id, name, token_hash, prefix, created_at, last_used_at
FROM auth_api_keys
WHERE user_id = ?
ORDER BY created_at DESC, id DESC
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := []appauth.APIKey{}
	for rows.Next() {
		key, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *AuthRepository) FindAPIKeyByTokenHash(ctx context.Context, tokenHash string) (appauth.APIKey, error) {
	return scanAPIKey(r.store.queryRow(ctx, `
SELECT id, user_id, name, token_hash, prefix, created_at, last_used_at
FROM auth_api_keys
WHERE token_hash = ?
`, tokenHash))
}

func (r *AuthRepository) UpdateAPIKeyLastUsed(ctx context.Context, id string, lastUsedAt time.Time) error {
	_, err := r.store.exec(ctx, `
UPDATE auth_api_keys
SET last_used_at = ?
WHERE id = ?
`, formatTime(lastUsedAt), id)
	return err
}

func (r *AuthRepository) DeleteAPIKey(ctx context.Context, userID string, id string) error {
	res, err := r.store.exec(ctx, `
DELETE FROM auth_api_keys
WHERE id = ? AND user_id = ?
`, id, userID)
	if err != nil {
		return err
	}
	if rows, err := res.RowsAffected(); err == nil && rows == 0 {
		return domain.ErrNotFound
	}
	return nil
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

func scanAPIKey(scanner interface {
	Scan(dest ...any) error
}) (appauth.APIKey, error) {
	var key appauth.APIKey
	var createdAt string
	var lastUsedAt sql.NullString
	if err := scanner.Scan(&key.ID, &key.UserID, &key.Name, &key.TokenHash, &key.Prefix, &createdAt, &lastUsedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appauth.APIKey{}, domain.ErrNotFound
		}
		return appauth.APIKey{}, err
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return appauth.APIKey{}, err
	}
	key.CreatedAt = parsedCreatedAt
	if lastUsedAt.Valid {
		parsedLastUsedAt, err := time.Parse(time.RFC3339Nano, lastUsedAt.String)
		if err != nil {
			return appauth.APIKey{}, err
		}
		key.LastUsedAt = &parsedLastUsedAt
	}
	return key, nil
}

func scanSession(scanner interface {
	Scan(dest ...any) error
}) (appauth.Session, error) {
	var session appauth.Session
	var expiresAt string
	var createdAt string
	if err := scanner.Scan(&session.ID, &session.UserID, &session.TokenHash, &session.Kind, &expiresAt, &createdAt); err != nil {
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

func formatOptionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}
