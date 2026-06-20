package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

func TestCreateUserCreatesFirstUser(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	service := NewService(repo)
	service.now = func() time.Time { return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC) }

	user, err := service.CreateUser(ctx, CreateUserCommand{
		Email:    " Admin@Example.COM ",
		Password: "strong-password",
	})
	if err != nil {
		t.Fatalf("create bootstrap user: %v", err)
	}
	if user.ID == "" || user.Email != "admin@example.com" {
		t.Fatalf("user = %#v", user)
	}

	stored := repo.usersByID[user.ID]
	if stored.PasswordHash == "" || stored.PasswordHash == "strong-password" {
		t.Fatalf("stored password hash = %q", stored.PasswordHash)
	}
}

func TestCreateUserAllowsAdditionalUsers(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	service := NewService(repo)

	if _, err := service.CreateUser(ctx, CreateUserCommand{Email: "admin@example.com", Password: "password-one"}); err != nil {
		t.Fatalf("create first user: %v", err)
	}

	other, err := service.CreateUser(ctx, CreateUserCommand{Email: "other@example.com", Password: "password-two"})
	if err != nil {
		t.Fatalf("create second user: %v", err)
	}
	if other.ID == "" || other.Email != "other@example.com" {
		t.Fatalf("second user = %#v", other)
	}
}

func TestLoginCreatesSession(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	service := NewService(repo)
	service.now = func() time.Time { return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC) }

	created, err := service.CreateUser(ctx, CreateUserCommand{Email: "admin@example.com", Password: "strong-password"})
	if err != nil {
		t.Fatalf("create bootstrap user: %v", err)
	}

	login, err := service.Login(ctx, LoginCommand{Email: "admin@example.com", Password: "strong-password"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if login.AccessToken == "" || login.RefreshToken == "" || login.TokenType != "Bearer" || !login.AccessExpiresAt.After(service.now()) || !login.RefreshExpiresAt.After(login.AccessExpiresAt) {
		t.Fatalf("login = %#v", login)
	}

	authenticated, err := service.AuthenticateSession(ctx, login.AccessToken)
	if err != nil {
		t.Fatalf("authenticate session: %v", err)
	}
	if authenticated.ID != created.ID {
		t.Fatalf("authenticated user = %#v, want %#v", authenticated, created)
	}

	refreshed, err := service.RefreshSession(ctx, login.RefreshToken)
	if err != nil {
		t.Fatalf("refresh session: %v", err)
	}
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" || refreshed.AccessToken == login.AccessToken || refreshed.RefreshToken == login.RefreshToken {
		t.Fatalf("refreshed session = %#v, original = %#v", refreshed, login)
	}
	if _, err := service.RefreshSession(ctx, login.RefreshToken); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("reused refresh token error = %v, want ErrUnauthorized", err)
	}

	if err := service.DeleteSession(ctx, refreshed.AccessToken); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	_, err = service.AuthenticateSession(ctx, refreshed.AccessToken)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("deleted session auth error = %v, want ErrUnauthorized", err)
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	service := NewService(repo)

	if _, err := service.CreateUser(ctx, CreateUserCommand{Email: "admin@example.com", Password: "strong-password"}); err != nil {
		t.Fatalf("create bootstrap user: %v", err)
	}

	_, err := service.Login(ctx, LoginCommand{Email: "admin@example.com", Password: "wrong-password"})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("login error = %v, want ErrUnauthorized", err)
	}
}

func TestLoginWithExternalIdentityCreatesAndReusesUser(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	service := NewService(repo)
	service.now = func() time.Time { return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC) }

	login, err := service.LoginWithExternalIdentity(ctx, ExternalIdentityLoginCommand{
		Provider:      "google",
		Subject:       "google-subject",
		Email:         " Admin@Example.COM ",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("external login: %v", err)
	}
	if login.User.ID == "" || login.User.Email != "admin@example.com" || login.AccessToken == "" || login.RefreshToken == "" {
		t.Fatalf("login = %#v", login)
	}

	second, err := service.LoginWithExternalIdentity(ctx, ExternalIdentityLoginCommand{
		Provider: "google",
		Subject:  "google-subject",
	})
	if err != nil {
		t.Fatalf("second external login: %v", err)
	}
	if second.User.ID != login.User.ID {
		t.Fatalf("second user = %#v, want %s", second.User, login.User.ID)
	}
}

func TestLoginWithExternalIdentityLinksVerifiedEmail(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	service := NewService(repo)

	user, err := service.CreateUser(ctx, CreateUserCommand{Email: "admin@example.com", Password: "strong-password"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	login, err := service.LoginWithExternalIdentity(ctx, ExternalIdentityLoginCommand{
		Provider:      "google",
		Subject:       "google-subject",
		Email:         "admin@example.com",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("external login: %v", err)
	}
	if login.User.ID != user.ID {
		t.Fatalf("linked user = %#v, want %s", login.User, user.ID)
	}
}

func TestLoginWithExternalIdentityRejectsUnverifiedEmail(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	service := NewService(repo)

	_, err := service.LoginWithExternalIdentity(ctx, ExternalIdentityLoginCommand{
		Provider:      "google",
		Subject:       "google-subject",
		Email:         "admin@example.com",
		EmailVerified: false,
	})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("external login error = %v, want ErrUnauthorized", err)
	}
}

func TestAPIKeyFlow(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	service := NewService(repo)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	user, err := service.CreateUser(ctx, CreateUserCommand{Email: "admin@example.com", Password: "strong-password"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ctx = WithWorkspaceID(ctx, user.WorkspaceID)

	created, err := service.CreateAPIKey(ctx, CreateAPIKeyCommand{UserID: user.ID, Name: "sdk"})
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	if created.ID == "" || created.Name != "sdk" || created.Key == "" || created.Prefix == "" {
		t.Fatalf("created key = %#v", created)
	}
	if strings.Join(created.Scopes, ",") != "usage:write,usage:read,meters:read,meters:write" {
		t.Fatalf("created scopes = %#v", created.Scopes)
	}
	if !strings.HasPrefix(created.Key, created.Prefix) {
		t.Fatalf("prefix looks wrong: %q key %q", created.Prefix, created.Key)
	}

	keys, err := service.ListAPIKeys(ctx, user.ID)
	if err != nil {
		t.Fatalf("list api keys: %v", err)
	}
	if len(keys) != 1 || keys[0].ID != created.ID || keys[0].LastUsedAt != nil {
		t.Fatalf("keys = %#v", keys)
	}
	if strings.Join(keys[0].Scopes, ",") != strings.Join(created.Scopes, ",") {
		t.Fatalf("listed scopes = %#v, want %#v", keys[0].Scopes, created.Scopes)
	}

	authenticated, err := service.AuthenticateAPIKey(ctx, created.Key)
	if err != nil {
		t.Fatalf("authenticate api key: %v", err)
	}
	if authenticated.ID != user.ID {
		t.Fatalf("authenticated = %#v, want user %s", authenticated, user.ID)
	}
	if repo.apiKeysByID[created.ID].LastUsedAt == nil {
		t.Fatal("last used was not updated")
	}

	if err := service.DeleteAPIKey(ctx, user.ID, created.ID); err != nil {
		t.Fatalf("delete api key: %v", err)
	}
	if _, err := service.AuthenticateAPIKey(ctx, created.Key); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("deleted api key auth error = %v, want ErrUnauthorized", err)
	}
}

func TestAPIKeyAuthenticationRejectsExpiredKeys(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	service := NewService(repo)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	user, err := service.CreateUser(ctx, CreateUserCommand{Email: "admin@example.com", Password: "strong-password"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ctx = WithWorkspaceID(ctx, user.WorkspaceID)

	expiresAt := now.Add(time.Hour)
	created, err := service.CreateAPIKey(ctx, CreateAPIKeyCommand{UserID: user.ID, Name: "short-lived", ExpiresAt: &expiresAt})
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	service.now = func() time.Time { return expiresAt.Add(time.Second) }
	if _, err := service.AuthenticateAPIKey(ctx, created.Key); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expired api key auth error = %v, want ErrUnauthorized", err)
	}
}

func TestCasbinAuthorizerEnforcesScopesAndMeters(t *testing.T) {
	ctx := context.Background()
	authorizer, err := NewCasbinAuthorizer()
	if err != nil {
		t.Fatalf("create authorizer: %v", err)
	}

	principal := Principal{
		Kind:          PrincipalKindAPIKey,
		ID:            "key_123",
		Scopes:        []string{string(ActionUsageWrite)},
		AllowedMeters: []string{"api_calls"},
	}

	if err := authorizer.Can(ctx, principal, ActionUsageWrite, Resource{Type: ResourceUsage, Meter: "api_calls"}); err != nil {
		t.Fatalf("usage write allowed error = %v", err)
	}
	if err := authorizer.Can(ctx, principal, ActionUsageRead, Resource{Type: ResourceUsage, Meter: "api_calls"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("usage read error = %v, want ErrForbidden", err)
	}
	if err := authorizer.Can(ctx, principal, ActionUsageWrite, Resource{Type: ResourceUsage, Meter: "other_meter"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("meter restriction error = %v, want ErrForbidden", err)
	}
	if err := authorizer.Can(ctx, principal, ActionUsageWrite, Resource{Type: ResourceUsage}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("empty meter restriction error = %v, want ErrForbidden", err)
	}

	session := Principal{Kind: PrincipalKindSession, ID: "user_123"}
	if err := authorizer.Can(ctx, session, ActionAlertsWrite, Resource{Type: ResourceAlert, Meter: "other_meter"}); err != nil {
		t.Fatalf("session write error = %v", err)
	}
}

type fakeRepository struct {
	usersByID           map[string]User
	userIDByEmail       map[string]string
	workspacesByID      map[string]Workspace
	membershipsByUserID map[string][]WorkspaceMembership
	identitiesByKey     map[string]Identity
	sessionsByHash      map[string]Session
	apiKeysByID         map[string]APIKey
	apiKeyIDByHash      map[string]string
	saveSessionError    error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		usersByID:           map[string]User{},
		userIDByEmail:       map[string]string{},
		workspacesByID:      map[string]Workspace{},
		membershipsByUserID: map[string][]WorkspaceMembership{},
		identitiesByKey:     map[string]Identity{},
		sessionsByHash:      map[string]Session{},
		apiKeysByID:         map[string]APIKey{},
		apiKeyIDByHash:      map[string]string{},
	}
}

func (r *fakeRepository) SaveUser(_ context.Context, user User) (User, error) {
	r.usersByID[user.ID] = user
	r.userIDByEmail[user.Email] = user.ID
	return user, nil
}

func (r *fakeRepository) SaveWorkspace(_ context.Context, workspace Workspace) (Workspace, error) {
	r.workspacesByID[workspace.ID] = workspace
	return workspace, nil
}

func (r *fakeRepository) SaveWorkspaceMembership(_ context.Context, membership WorkspaceMembership) (WorkspaceMembership, error) {
	r.membershipsByUserID[membership.UserID] = append(r.membershipsByUserID[membership.UserID], membership)
	return membership, nil
}

func (r *fakeRepository) FindDefaultWorkspaceByUserID(_ context.Context, userID string) (Workspace, error) {
	memberships := r.membershipsByUserID[userID]
	if len(memberships) == 0 {
		return Workspace{}, domain.ErrNotFound
	}
	workspace, ok := r.workspacesByID[memberships[0].WorkspaceID]
	if !ok {
		return Workspace{}, domain.ErrNotFound
	}
	return workspace, nil
}

func (r *fakeRepository) FindUserByID(_ context.Context, id string) (User, error) {
	user, ok := r.usersByID[id]
	if !ok {
		return User{}, domain.ErrNotFound
	}
	return user, nil
}

func (r *fakeRepository) FindUserByEmail(_ context.Context, email string) (User, error) {
	id, ok := r.userIDByEmail[email]
	if !ok {
		return User{}, domain.ErrNotFound
	}
	return r.usersByID[id], nil
}

func (r *fakeRepository) SaveIdentity(_ context.Context, identity Identity) (Identity, error) {
	r.identitiesByKey[identity.Provider+"|"+identity.Subject] = identity
	return identity, nil
}

func (r *fakeRepository) FindIdentityByProviderSubject(_ context.Context, provider string, subject string) (Identity, error) {
	identity, ok := r.identitiesByKey[provider+"|"+subject]
	if !ok {
		return Identity{}, domain.ErrNotFound
	}
	return identity, nil
}

func (r *fakeRepository) SaveSession(_ context.Context, session Session) (Session, error) {
	if r.saveSessionError != nil {
		return Session{}, r.saveSessionError
	}
	r.sessionsByHash[session.TokenHash] = session
	return session, nil
}

func (r *fakeRepository) FindSessionByTokenHash(_ context.Context, tokenHash string, kind string, now time.Time) (Session, error) {
	session, ok := r.sessionsByHash[tokenHash]
	if !ok || session.Kind != kind || !session.ExpiresAt.After(now) {
		return Session{}, domain.ErrNotFound
	}
	return session, nil
}

func (r *fakeRepository) DeleteSessionByTokenHash(_ context.Context, tokenHash string) error {
	delete(r.sessionsByHash, tokenHash)
	return nil
}

func (r *fakeRepository) SaveAPIKey(_ context.Context, key APIKey) (APIKey, error) {
	r.apiKeysByID[key.ID] = key
	r.apiKeyIDByHash[key.TokenHash] = key.ID
	return key, nil
}

func (r *fakeRepository) ListAPIKeys(ctx context.Context, userID string) ([]APIKey, error) {
	workspaceID, err := RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	keys := []APIKey{}
	for _, key := range r.apiKeysByID {
		if key.UserID == userID && key.WorkspaceID == workspaceID {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (r *fakeRepository) FindAPIKeyByTokenHash(_ context.Context, tokenHash string) (APIKey, error) {
	id, ok := r.apiKeyIDByHash[tokenHash]
	if !ok {
		return APIKey{}, domain.ErrNotFound
	}
	return r.apiKeysByID[id], nil
}

func (r *fakeRepository) UpdateAPIKeyLastUsed(_ context.Context, id string, lastUsedAt time.Time) error {
	key, ok := r.apiKeysByID[id]
	if !ok {
		return domain.ErrNotFound
	}
	key.LastUsedAt = &lastUsedAt
	r.apiKeysByID[id] = key
	return nil
}

func (r *fakeRepository) DeleteAPIKey(ctx context.Context, userID string, id string) error {
	workspaceID, err := RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	key, ok := r.apiKeysByID[id]
	if !ok || key.UserID != userID || key.WorkspaceID != workspaceID {
		return domain.ErrNotFound
	}
	delete(r.apiKeyIDByHash, key.TokenHash)
	delete(r.apiKeysByID, id)
	return nil
}
