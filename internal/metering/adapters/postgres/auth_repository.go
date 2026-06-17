package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/postgres/postgresdb"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type AuthRepository struct {
	queries *postgresdb.Queries
}

func NewAuthRepository(store *Store) *AuthRepository {
	return &AuthRepository{queries: postgresdb.New(store)}
}

func (r *AuthRepository) CountUsers(ctx context.Context) (int, error) {
	count, err := queriesFor(ctx, r.queries).CountUsers(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *AuthRepository) SaveUser(ctx context.Context, user appauth.User) (appauth.User, error) {
	err := queriesFor(ctx, r.queries).SaveUser(ctx, postgresdb.SaveUserParams{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		CreatedAt:    formatTime(user.CreatedAt),
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return appauth.User{}, errors.Join(domain.ErrConflict, err)
		}
		return appauth.User{}, err
	}
	return user, nil
}

func (r *AuthRepository) FindUserByID(ctx context.Context, id string) (appauth.User, error) {
	user, err := queriesFor(ctx, r.queries).FindUserByID(ctx, id)
	return userFromFields(user.ID, user.Email, user.PasswordHash, user.CreatedAt, err)
}

func (r *AuthRepository) FindUserByEmail(ctx context.Context, email string) (appauth.User, error) {
	user, err := queriesFor(ctx, r.queries).FindUserByEmail(ctx, email)
	return userFromFields(user.ID, user.Email, user.PasswordHash, user.CreatedAt, err)
}

func (r *AuthRepository) SaveSession(ctx context.Context, session appauth.Session) (appauth.Session, error) {
	err := queriesFor(ctx, r.queries).SaveSession(ctx, postgresdb.SaveSessionParams{
		ID:        session.ID,
		UserID:    session.UserID,
		TokenHash: session.TokenHash,
		Kind:      session.Kind,
		ExpiresAt: formatTime(session.ExpiresAt),
		CreatedAt: formatTime(session.CreatedAt),
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return appauth.Session{}, errors.Join(domain.ErrConflict, err)
		}
		return appauth.Session{}, err
	}
	return session, nil
}

func (r *AuthRepository) FindSessionByTokenHash(ctx context.Context, tokenHash string, kind string, now time.Time) (appauth.Session, error) {
	session, err := queriesFor(ctx, r.queries).FindSessionByTokenHash(ctx, postgresdb.FindSessionByTokenHashParams{
		TokenHash: tokenHash,
		Kind:      kind,
		ExpiresAt: formatTime(now),
	})
	return sessionFromFields(session.ID, session.UserID, session.TokenHash, session.Kind, session.ExpiresAt, session.CreatedAt, err)
}

func (r *AuthRepository) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	return queriesFor(ctx, r.queries).DeleteSessionByTokenHash(ctx, tokenHash)
}

func (r *AuthRepository) SaveAPIKey(ctx context.Context, key appauth.APIKey) (appauth.APIKey, error) {
	err := queriesFor(ctx, r.queries).SaveAPIKey(ctx, postgresdb.SaveAPIKeyParams{
		ID:         key.ID,
		UserID:     key.UserID,
		Name:       key.Name,
		TokenHash:  key.TokenHash,
		Prefix:     key.Prefix,
		CreatedAt:  formatTime(key.CreatedAt),
		LastUsedAt: formatOptionalTime(key.LastUsedAt),
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return appauth.APIKey{}, errors.Join(domain.ErrConflict, err)
		}
		return appauth.APIKey{}, err
	}
	return key, nil
}

func (r *AuthRepository) ListAPIKeys(ctx context.Context, userID string) ([]appauth.APIKey, error) {
	rows, err := queriesFor(ctx, r.queries).ListAPIKeys(ctx, userID)
	if err != nil {
		return nil, err
	}

	keys := make([]appauth.APIKey, 0, len(rows))
	for _, row := range rows {
		key, err := apiKeyFromFields(row.ID, row.UserID, row.Name, row.TokenHash, row.Prefix, row.CreatedAt, row.LastUsedAt, nil)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (r *AuthRepository) FindAPIKeyByTokenHash(ctx context.Context, tokenHash string) (appauth.APIKey, error) {
	key, err := queriesFor(ctx, r.queries).FindAPIKeyByTokenHash(ctx, tokenHash)
	return apiKeyFromFields(key.ID, key.UserID, key.Name, key.TokenHash, key.Prefix, key.CreatedAt, key.LastUsedAt, err)
}

func (r *AuthRepository) UpdateAPIKeyLastUsed(ctx context.Context, id string, lastUsedAt time.Time) error {
	return queriesFor(ctx, r.queries).UpdateAPIKeyLastUsed(ctx, postgresdb.UpdateAPIKeyLastUsedParams{
		LastUsedAt: sql.NullString{String: formatTime(lastUsedAt), Valid: true},
		ID:         id,
	})
}

func (r *AuthRepository) DeleteAPIKey(ctx context.Context, userID string, id string) error {
	rows, err := queriesFor(ctx, r.queries).DeleteAPIKey(ctx, postgresdb.DeleteAPIKeyParams{ID: id, UserID: userID})
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func userFromFields(id string, email string, passwordHash string, createdAt string, err error) (appauth.User, error) {
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appauth.User{}, domain.ErrNotFound
		}
		return appauth.User{}, err
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return appauth.User{}, err
	}
	return appauth.User{ID: id, Email: email, PasswordHash: passwordHash, CreatedAt: parsedCreatedAt}, nil
}

func apiKeyFromFields(id string, userID string, name string, tokenHash string, prefix string, createdAt string, lastUsedAt sql.NullString, err error) (appauth.APIKey, error) {
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appauth.APIKey{}, domain.ErrNotFound
		}
		return appauth.APIKey{}, err
	}

	key := appauth.APIKey{
		ID:        id,
		UserID:    userID,
		Name:      name,
		TokenHash: tokenHash,
		Prefix:    prefix,
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

func sessionFromFields(id string, userID string, tokenHash string, kind string, expiresAt string, createdAt string, err error) (appauth.Session, error) {
	if err != nil {
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
	return appauth.Session{
		ID:        id,
		UserID:    userID,
		TokenHash: tokenHash,
		Kind:      kind,
		ExpiresAt: parsedExpiresAt,
		CreatedAt: parsedCreatedAt,
	}, nil
}

func formatOptionalTime(value *time.Time) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(*value), Valid: true}
}
