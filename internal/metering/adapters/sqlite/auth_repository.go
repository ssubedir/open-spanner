package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite/sqlitedb"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type AuthRepository struct {
	queries *sqlitedb.Queries
}

func NewAuthRepository(store *Store) *AuthRepository {
	return &AuthRepository{queries: sqlitedb.New(store)}
}

func (r *AuthRepository) CountUsers(ctx context.Context) (int, error) {
	count, err := queriesFor(ctx, r.queries).CountUsers(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *AuthRepository) SaveUser(ctx context.Context, user appauth.User) (appauth.User, error) {
	err := queriesFor(ctx, r.queries).SaveUser(ctx, sqlitedb.SaveUserParams{
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

func (r *AuthRepository) SaveIdentity(ctx context.Context, identity appauth.Identity) (appauth.Identity, error) {
	err := queriesFor(ctx, r.queries).SaveIdentity(ctx, sqlitedb.SaveIdentityParams{
		ID:            identity.ID,
		UserID:        identity.UserID,
		Provider:      identity.Provider,
		Subject:       identity.Subject,
		Email:         identity.Email,
		EmailVerified: authBoolInt(identity.EmailVerified),
		CreatedAt:     formatTime(identity.CreatedAt),
		UpdatedAt:     formatTime(identity.UpdatedAt),
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return appauth.Identity{}, errors.Join(domain.ErrConflict, err)
		}
		return appauth.Identity{}, err
	}
	return identity, nil
}

func (r *AuthRepository) FindIdentityByProviderSubject(ctx context.Context, provider string, subject string) (appauth.Identity, error) {
	identity, err := queriesFor(ctx, r.queries).FindIdentityByProviderSubject(ctx, sqlitedb.FindIdentityByProviderSubjectParams{
		Provider: provider,
		Subject:  subject,
	})
	return identityFromFields(identity.ID, identity.UserID, identity.Provider, identity.Subject, identity.Email, identity.EmailVerified != 0, identity.CreatedAt, identity.UpdatedAt, err)
}

func (r *AuthRepository) SaveSession(ctx context.Context, session appauth.Session) (appauth.Session, error) {
	err := queriesFor(ctx, r.queries).SaveSession(ctx, sqlitedb.SaveSessionParams{
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
	session, err := queriesFor(ctx, r.queries).FindSessionByTokenHash(ctx, sqlitedb.FindSessionByTokenHashParams{
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
	err := queriesFor(ctx, r.queries).SaveAPIKey(ctx, sqlitedb.SaveAPIKeyParams{
		ID:            key.ID,
		UserID:        key.UserID,
		Name:          key.Name,
		TokenHash:     key.TokenHash,
		Prefix:        key.Prefix,
		Scopes:        formatStringArray(key.Scopes),
		AllowedMeters: formatStringArray(key.AllowedMeters),
		ExpiresAt:     formatOptionalTime(key.ExpiresAt),
		RevokedAt:     formatOptionalTime(key.RevokedAt),
		CreatedAt:     formatTime(key.CreatedAt),
		LastUsedAt:    formatOptionalTime(key.LastUsedAt),
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
		key, err := apiKeyFromFields(row.ID, row.UserID, row.Name, row.TokenHash, row.Prefix, row.Scopes, row.AllowedMeters, row.ExpiresAt, row.RevokedAt, row.CreatedAt, row.LastUsedAt, nil)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (r *AuthRepository) FindAPIKeyByTokenHash(ctx context.Context, tokenHash string) (appauth.APIKey, error) {
	key, err := queriesFor(ctx, r.queries).FindAPIKeyByTokenHash(ctx, tokenHash)
	return apiKeyFromFields(key.ID, key.UserID, key.Name, key.TokenHash, key.Prefix, key.Scopes, key.AllowedMeters, key.ExpiresAt, key.RevokedAt, key.CreatedAt, key.LastUsedAt, err)
}

func (r *AuthRepository) UpdateAPIKeyLastUsed(ctx context.Context, id string, lastUsedAt time.Time) error {
	return queriesFor(ctx, r.queries).UpdateAPIKeyLastUsed(ctx, sqlitedb.UpdateAPIKeyLastUsedParams{
		LastUsedAt: sql.NullString{String: formatTime(lastUsedAt), Valid: true},
		ID:         id,
	})
}

func (r *AuthRepository) DeleteAPIKey(ctx context.Context, userID string, id string) error {
	rows, err := queriesFor(ctx, r.queries).DeleteAPIKey(ctx, sqlitedb.DeleteAPIKeyParams{ID: id, UserID: userID})
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

func identityFromFields(id string, userID string, provider string, subject string, email string, emailVerified bool, createdAt string, updatedAt string, err error) (appauth.Identity, error) {
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appauth.Identity{}, domain.ErrNotFound
		}
		return appauth.Identity{}, err
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return appauth.Identity{}, err
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return appauth.Identity{}, err
	}
	return appauth.Identity{
		ID:            id,
		UserID:        userID,
		Provider:      provider,
		Subject:       subject,
		Email:         email,
		EmailVerified: emailVerified,
		CreatedAt:     parsedCreatedAt,
		UpdatedAt:     parsedUpdatedAt,
	}, nil
}

func authBoolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func apiKeyFromFields(id string, userID string, name string, tokenHash string, prefix string, scopesJSON string, allowedMetersJSON string, expiresAt sql.NullString, revokedAt sql.NullString, createdAt string, lastUsedAt sql.NullString, err error) (appauth.APIKey, error) {
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
	scopes, err := parseStringArray(scopesJSON)
	if err != nil {
		return appauth.APIKey{}, err
	}
	key.Scopes = scopes
	allowedMeters, err := parseStringArray(allowedMetersJSON)
	if err != nil {
		return appauth.APIKey{}, err
	}
	key.AllowedMeters = allowedMeters
	parsedExpiresAt, err := parseOptionalTime(expiresAt)
	if err != nil {
		return appauth.APIKey{}, err
	}
	key.ExpiresAt = parsedExpiresAt
	parsedRevokedAt, err := parseOptionalTime(revokedAt)
	if err != nil {
		return appauth.APIKey{}, err
	}
	key.RevokedAt = parsedRevokedAt
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return appauth.APIKey{}, err
	}
	key.CreatedAt = parsedCreatedAt
	parsedLastUsedAt, err := parseOptionalTime(lastUsedAt)
	if err != nil {
		return appauth.APIKey{}, err
	}
	key.LastUsedAt = parsedLastUsedAt
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

func parseOptionalTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func formatStringArray(values []string) string {
	data, _ := json.Marshal(values)
	return string(data)
}

func parseStringArray(value string) ([]string, error) {
	if value == "" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(value), &values); err != nil {
		return nil, err
	}
	return values, nil
}
