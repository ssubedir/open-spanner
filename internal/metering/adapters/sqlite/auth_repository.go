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
	store   *Store
}

func NewAuthRepository(store *Store) *AuthRepository {
	return &AuthRepository{queries: sqlitedb.New(store), store: store}
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

func (r *AuthRepository) SaveWorkspace(ctx context.Context, workspace appauth.Workspace) (appauth.Workspace, error) {
	err := queriesFor(ctx, r.queries).SaveWorkspace(ctx, sqlitedb.SaveWorkspaceParams{
		ID:        workspace.ID,
		Name:      workspace.Name,
		CreatedAt: formatTime(workspace.CreatedAt),
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return appauth.Workspace{}, errors.Join(domain.ErrConflict, err)
		}
		return appauth.Workspace{}, err
	}
	return workspace, nil
}

func (r *AuthRepository) SaveWorkspaceMembership(ctx context.Context, membership appauth.WorkspaceMembership) (appauth.WorkspaceMembership, error) {
	err := queriesFor(ctx, r.queries).SaveWorkspaceMembership(ctx, sqlitedb.SaveWorkspaceMembershipParams{
		WorkspaceID: membership.WorkspaceID,
		UserID:      membership.UserID,
		Role:        membership.Role,
		CreatedAt:   formatTime(membership.CreatedAt),
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return appauth.WorkspaceMembership{}, errors.Join(domain.ErrConflict, err)
		}
		return appauth.WorkspaceMembership{}, err
	}
	return membership, nil
}

func (r *AuthRepository) FindDefaultWorkspaceByUserID(ctx context.Context, userID string) (appauth.Workspace, error) {
	workspace, err := queriesFor(ctx, r.queries).FindDefaultWorkspaceByUserID(ctx, userID)
	return workspaceFromFields(workspace.ID, workspace.Name, workspace.CreatedAt, err)
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
		ID:          session.ID,
		UserID:      session.UserID,
		WorkspaceID: session.WorkspaceID,
		TokenHash:   session.TokenHash,
		Kind:        session.Kind,
		ExpiresAt:   formatTime(session.ExpiresAt),
		CreatedAt:   formatTime(session.CreatedAt),
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
	return sessionFromFields(session.ID, session.UserID, session.WorkspaceID, session.TokenHash, session.Kind, session.ExpiresAt, session.CreatedAt, err)
}

func (r *AuthRepository) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	return queriesFor(ctx, r.queries).DeleteSessionByTokenHash(ctx, tokenHash)
}

func (r *AuthRepository) CreateAPIKey(ctx context.Context, key appauth.APIKey, event appauth.APIKeyEvent) (appauth.APIKey, error) {
	err := r.store.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := r.saveAPIKey(txCtx, key); err != nil {
			return err
		}
		return r.saveAPIKeyEvent(txCtx, event)
	})
	return key, err
}

func (r *AuthRepository) saveAPIKey(ctx context.Context, key appauth.APIKey) error {
	err := queriesFor(ctx, r.queries).SaveAPIKey(ctx, sqlitedb.SaveAPIKeyParams{
		ID:            key.ID,
		UserID:        key.UserID,
		WorkspaceID:   key.WorkspaceID,
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
			return errors.Join(domain.ErrConflict, err)
		}
		return err
	}
	return nil
}

func (r *AuthRepository) ListAPIKeys(ctx context.Context, userID string) ([]appauth.APIKey, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListAPIKeys(ctx, sqlitedb.ListAPIKeysParams{
		UserID:      userID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, err
	}

	keys := make([]appauth.APIKey, 0, len(rows))
	for _, row := range rows {
		key, err := apiKeyFromFields(row.ID, row.UserID, row.WorkspaceID, row.Name, row.TokenHash, row.Prefix, row.Scopes, row.AllowedMeters, row.ExpiresAt, row.RevokedAt, row.CreatedAt, row.LastUsedAt, nil)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (r *AuthRepository) FindAPIKeyPrincipalByTokenHash(ctx context.Context, tokenHash string) (appauth.APIKey, appauth.User, error) {
	row, err := queriesFor(ctx, r.queries).FindAPIKeyPrincipalByTokenHash(ctx, tokenHash)
	key, err := apiKeyFromFields(row.KeyID, row.KeyUserID, row.KeyWorkspaceID, row.KeyName, row.KeyTokenHash, row.KeyPrefix, row.KeyScopes, row.KeyAllowedMeters, row.KeyExpiresAt, row.KeyRevokedAt, row.KeyCreatedAt, row.KeyLastUsedAt, err)
	if err != nil {
		return appauth.APIKey{}, appauth.User{}, err
	}
	user, err := userFromFields(row.UserID, row.UserEmail, "", row.UserCreatedAt, nil)
	return key, user, err
}

func (r *AuthRepository) FindAPIKeyByID(ctx context.Context, userID string, id string) (appauth.APIKey, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return appauth.APIKey{}, err
	}
	key, err := queriesFor(ctx, r.queries).FindAPIKeyByID(ctx, sqlitedb.FindAPIKeyByIDParams{ID: id, UserID: userID, WorkspaceID: workspaceID})
	return apiKeyFromFields(key.ID, key.UserID, key.WorkspaceID, key.Name, key.TokenHash, key.Prefix, key.Scopes, key.AllowedMeters, key.ExpiresAt, key.RevokedAt, key.CreatedAt, key.LastUsedAt, err)
}

func (r *AuthRepository) UpdateAPIKeyLastUsed(ctx context.Context, id string, lastUsedAt time.Time) error {
	return queriesFor(ctx, r.queries).UpdateAPIKeyLastUsed(ctx, sqlitedb.UpdateAPIKeyLastUsedParams{
		LastUsedAt: sql.NullString{String: formatTime(lastUsedAt), Valid: true},
		ID:         id,
	})
}

func (r *AuthRepository) RotateAPIKey(ctx context.Context, userID string, sourceID string, replacement appauth.APIKey, revokeAt time.Time, events []appauth.APIKeyEvent) (appauth.APIKey, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return appauth.APIKey{}, err
	}
	err = r.store.WithinTransaction(ctx, func(txCtx context.Context) error {
		rows, err := queriesFor(txCtx, r.queries).ScheduleAPIKeyRevocation(txCtx, sqlitedb.ScheduleAPIKeyRevocationParams{RevokedAt: formatOptionalTime(&revokeAt), ID: sourceID, UserID: userID, WorkspaceID: workspaceID})
		if err != nil {
			return err
		}
		if rows == 0 {
			return errors.Join(domain.ErrConflict, errors.New("api key is already revoked or rotating"))
		}
		if err := r.saveAPIKey(txCtx, replacement); err != nil {
			return err
		}
		for _, event := range events {
			if err := r.saveAPIKeyEvent(txCtx, event); err != nil {
				return err
			}
		}
		return nil
	})
	return replacement, err
}

func (r *AuthRepository) RevokeAPIKey(ctx context.Context, userID string, id string, revokedAt time.Time, event appauth.APIKeyEvent) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	return r.store.WithinTransaction(ctx, func(txCtx context.Context) error {
		value := formatOptionalTime(&revokedAt)
		rows, err := queriesFor(txCtx, r.queries).RevokeAPIKey(txCtx, sqlitedb.RevokeAPIKeyParams{RevokedAt: value, ID: id, UserID: userID, WorkspaceID: workspaceID, RevokedAt_2: value})
		if err != nil {
			return err
		}
		if rows == 0 {
			return domain.ErrNotFound
		}
		return r.saveAPIKeyEvent(txCtx, event)
	})
}

func (r *AuthRepository) ListAPIKeyEvents(ctx context.Context, userID string, limit int) ([]appauth.APIKeyEvent, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListAPIKeyEvents(ctx, sqlitedb.ListAPIKeyEventsParams{WorkspaceID: workspaceID, UserID: userID, Limit: int64(limit)})
	if err != nil {
		return nil, err
	}
	events := make([]appauth.APIKeyEvent, 0, len(rows))
	for _, row := range rows {
		effectiveAt, err := parseOptionalTime(row.EffectiveAt)
		if err != nil {
			return nil, err
		}
		createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
		if err != nil {
			return nil, err
		}
		events = append(events, appauth.APIKeyEvent{ID: row.ID, WorkspaceID: row.WorkspaceID, UserID: row.UserID, APIKeyID: row.ApiKeyID, KeyName: row.KeyName, KeyPrefix: row.KeyPrefix, EventType: row.EventType, RelatedAPIKeyID: row.RelatedApiKeyID.String, EffectiveAt: effectiveAt, CreatedAt: createdAt})
	}
	return events, nil
}

func (r *AuthRepository) saveAPIKeyEvent(ctx context.Context, event appauth.APIKeyEvent) error {
	return queriesFor(ctx, r.queries).SaveAPIKeyEvent(ctx, sqlitedb.SaveAPIKeyEventParams{ID: event.ID, WorkspaceID: event.WorkspaceID, UserID: event.UserID, ApiKeyID: event.APIKeyID, KeyName: event.KeyName, KeyPrefix: event.KeyPrefix, EventType: event.EventType, RelatedApiKeyID: sql.NullString{String: event.RelatedAPIKeyID, Valid: event.RelatedAPIKeyID != ""}, EffectiveAt: formatOptionalTime(event.EffectiveAt), CreatedAt: formatTime(event.CreatedAt)})
}

func workspaceFromFields(id string, name string, createdAt string, err error) (appauth.Workspace, error) {
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appauth.Workspace{}, domain.ErrNotFound
		}
		return appauth.Workspace{}, err
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return appauth.Workspace{}, err
	}
	return appauth.Workspace{ID: id, Name: name, CreatedAt: parsedCreatedAt}, nil
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

func apiKeyFromFields(id string, userID string, workspaceID string, name string, tokenHash string, prefix string, scopesJSON string, allowedMetersJSON string, expiresAt sql.NullString, revokedAt sql.NullString, createdAt string, lastUsedAt sql.NullString, err error) (appauth.APIKey, error) {
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appauth.APIKey{}, domain.ErrNotFound
		}
		return appauth.APIKey{}, err
	}

	key := appauth.APIKey{
		ID:          id,
		UserID:      userID,
		WorkspaceID: workspaceID,
		Name:        name,
		TokenHash:   tokenHash,
		Prefix:      prefix,
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

func sessionFromFields(id string, userID string, workspaceID string, tokenHash string, kind string, expiresAt string, createdAt string, err error) (appauth.Session, error) {
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
		ID:          id,
		UserID:      userID,
		WorkspaceID: workspaceID,
		TokenHash:   tokenHash,
		Kind:        kind,
		ExpiresAt:   parsedExpiresAt,
		CreatedAt:   parsedCreatedAt,
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
