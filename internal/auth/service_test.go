package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
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

func TestLoginWithExternalIdentityHonorsDisabledRegistration(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	service := NewService(repo)

	_, err := service.LoginWithExternalIdentity(ctx, ExternalIdentityLoginCommand{
		Provider:             "github",
		Subject:              "new-user",
		Email:                "new-user@example.com",
		EmailVerified:        true,
		RegistrationDisabled: true,
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("external identity login error = %v, want forbidden", err)
	}

	user, err := service.CreateUser(ctx, CreateUserCommand{Email: "existing@example.com", Password: "strong-password"})
	if err != nil {
		t.Fatalf("create existing user: %v", err)
	}
	login, err := service.LoginWithExternalIdentity(ctx, ExternalIdentityLoginCommand{
		Provider:             "github",
		Subject:              "existing-user",
		Email:                user.Email,
		EmailVerified:        true,
		RegistrationDisabled: true,
	})
	if err != nil || login.User.ID != user.ID {
		t.Fatalf("link existing external identity login = %#v err=%v", login, err)
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
	if strings.Join(created.Scopes, ",") != "usage:write,usage:read,meters:read,meters:write,plans:read" {
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

func TestAPIKeyAuthenticationCoalescesLastUsedUpdates(t *testing.T) {
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

	for range 3 {
		if _, err := service.AuthenticateAPIKey(ctx, created.Key); err != nil {
			t.Fatalf("authenticate api key: %v", err)
		}
	}
	if repo.apiKeyPrincipalLookups != 3 {
		t.Fatalf("principal lookups = %d, want 3", repo.apiKeyPrincipalLookups)
	}
	if repo.lastUsedUpdates != 1 {
		t.Fatalf("last-used updates = %d, want 1", repo.lastUsedUpdates)
	}

	now = now.Add(apiKeyLastUsedInterval)
	if _, err := service.AuthenticateAPIKey(ctx, created.Key); err != nil {
		t.Fatalf("authenticate after interval: %v", err)
	}
	if repo.lastUsedUpdates != 2 {
		t.Fatalf("last-used updates after interval = %d, want 2", repo.lastUsedUpdates)
	}
}

func TestAPIKeyUsageTrackerCoalescesConcurrentUpdates(t *testing.T) {
	tracker := &apiKeyUsageTracker{lastUpdates: make(map[string]time.Time)}
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	key := APIKey{ID: "key-1"}
	var updates atomic.Int32
	var wg sync.WaitGroup

	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tracker.shouldUpdate(key, now) {
				updates.Add(1)
			}
		}()
	}
	wg.Wait()
	if updates.Load() != 1 {
		t.Fatalf("concurrent update reservations = %d, want 1", updates.Load())
	}

	tracker.release(key.ID, now)
	if !tracker.shouldUpdate(key, now) {
		t.Fatal("released reservation was not retryable")
	}
}

func TestAPIKeyAuthenticationRetriesFailedLastUsedUpdate(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	service := NewService(repo)
	service.now = func() time.Time { return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC) }

	user, err := service.CreateUser(ctx, CreateUserCommand{Email: "admin@example.com", Password: "strong-password"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ctx = WithWorkspaceID(ctx, user.WorkspaceID)
	created, err := service.CreateAPIKey(ctx, CreateAPIKeyCommand{UserID: user.ID, Name: "sdk"})
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	repo.updateLastUsedError = errors.New("database unavailable")
	if _, err := service.AuthenticateAPIKey(ctx, created.Key); err == nil {
		t.Fatal("authenticate error = nil, want last-used update error")
	}
	if _, err := service.AuthenticateAPIKey(ctx, created.Key); err != nil {
		t.Fatalf("retry authenticate api key: %v", err)
	}
	if repo.lastUsedUpdates != 2 {
		t.Fatalf("last-used update attempts = %d, want 2", repo.lastUsedUpdates)
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
		Role:          RoleOwner,
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
	systemReader := Principal{Kind: PrincipalKindAPIKey, ID: "key_system_read", Role: RoleOwner, Scopes: []string{string(ActionSystemRead)}}
	if err := authorizer.Can(ctx, systemReader, ActionSystemRead, Resource{Type: ResourceSystem}); err != nil {
		t.Fatalf("system read error = %v", err)
	}
	if err := authorizer.Can(ctx, systemReader, ActionSystemWrite, Resource{Type: ResourceSystem}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("system write with read-only key error = %v, want ErrForbidden", err)
	}

	session := Principal{Kind: PrincipalKindSession, ID: "user_123", Role: RoleOwner}
	if err := authorizer.Can(ctx, session, ActionAlertsWrite, Resource{Type: ResourceAlert, Meter: "other_meter"}); err != nil {
		t.Fatalf("session write error = %v", err)
	}
}

func TestWorkspaceInvitationAcceptanceAndSwitching(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	service := NewService(repo)
	now := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	owner, err := service.CreateUser(ctx, CreateUserCommand{Email: "owner@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerLogin, err := service.Login(ctx, LoginCommand{Email: owner.Email, Password: "password123"})
	if err != nil {
		t.Fatalf("login owner: %v", err)
	}
	ownerPrincipal, err := service.AuthenticateSessionPrincipal(ctx, ownerLogin.AccessToken)
	if err != nil {
		t.Fatalf("authenticate owner: %v", err)
	}
	if ownerPrincipal.Role != RoleOwner {
		t.Fatalf("owner role = %q", ownerPrincipal.Role)
	}

	invitation, err := service.CreateWorkspaceInvitation(ctx, ownerPrincipal, CreateWorkspaceInvitationCommand{Email: "member@example.com", Role: RoleViewer})
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	if invitation.Token == "" || invitation.Status != "pending" {
		t.Fatalf("invitation = %+v", invitation)
	}
	preview, err := service.PreviewWorkspaceInvitation(ctx, invitation.Token)
	if err != nil {
		t.Fatalf("preview invitation: %v", err)
	}
	if preview.Email != "member@example.com" || preview.WorkspaceID != ownerPrincipal.WorkspaceID {
		t.Fatalf("preview = %+v", preview)
	}

	member, err := service.CreateUser(ctx, CreateUserCommand{Email: "member@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	memberLogin, err := service.Login(ctx, LoginCommand{Email: member.Email, Password: "password123"})
	if err != nil {
		t.Fatalf("login member: %v", err)
	}
	memberPrincipal, err := service.AuthenticateSessionPrincipal(ctx, memberLogin.AccessToken)
	if err != nil {
		t.Fatalf("authenticate member: %v", err)
	}
	accepted, err := service.AcceptWorkspaceInvitation(ctx, memberPrincipal, invitation.Token)
	if err != nil {
		t.Fatalf("accept invitation: %v", err)
	}
	if accepted.User.WorkspaceID != ownerPrincipal.WorkspaceID || accepted.User.Role != RoleViewer {
		t.Fatalf("accepted user = %+v", accepted.User)
	}

	workspaces, err := service.ListWorkspaces(ctx, member.ID)
	if err != nil {
		t.Fatalf("list workspaces: %v", err)
	}
	if len(workspaces) != 2 {
		t.Fatalf("workspace count = %d, want 2", len(workspaces))
	}
	if _, err := service.AcceptWorkspaceInvitation(ctx, memberPrincipal, invitation.Token); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("accept reused invitation error = %v", err)
	}
}

func TestWorkspaceInvitationRejectsWrongEmailAndViewerWrites(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	service := NewService(repo)
	owner, _ := service.CreateUser(ctx, CreateUserCommand{Email: "owner@example.com", Password: "password123"})
	ownerLogin, _ := service.Login(ctx, LoginCommand{Email: owner.Email, Password: "password123"})
	ownerPrincipal, _ := service.AuthenticateSessionPrincipal(ctx, ownerLogin.AccessToken)
	invitation, err := service.CreateWorkspaceInvitation(ctx, ownerPrincipal, CreateWorkspaceInvitationCommand{Email: "expected@example.com", Role: RoleViewer})
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}

	other, _ := service.CreateUser(ctx, CreateUserCommand{Email: "other@example.com", Password: "password123"})
	otherLogin, _ := service.Login(ctx, LoginCommand{Email: other.Email, Password: "password123"})
	otherPrincipal, _ := service.AuthenticateSessionPrincipal(ctx, otherLogin.AccessToken)
	if _, err := service.AcceptWorkspaceInvitation(ctx, otherPrincipal, invitation.Token); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("wrong-email acceptance error = %v", err)
	}

	authorizer, err := NewCasbinAuthorizer()
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	viewer := Principal{Kind: PrincipalKindSession, Role: RoleViewer}
	if err := authorizer.Can(ctx, viewer, ActionMetersRead, Resource{Type: ResourceMeter}); err != nil {
		t.Fatalf("viewer read error = %v", err)
	}
	if err := authorizer.Can(ctx, viewer, ActionMetersWrite, Resource{Type: ResourceMeter}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("viewer write error = %v", err)
	}
	viewerKey := Principal{Kind: PrincipalKindAPIKey, Role: RoleViewer, Scopes: []string{"*"}}
	if err := authorizer.Can(ctx, viewerKey, ActionMetersRead, Resource{Type: ResourceMeter}); err != nil {
		t.Fatalf("viewer api key read error = %v", err)
	}
	if err := authorizer.Can(ctx, viewerKey, ActionMetersWrite, Resource{Type: ResourceMeter}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("viewer api key write error = %v", err)
	}
}

func TestWorkspaceCannotRemoveFinalOwner(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	service := NewService(repo)
	owner, _ := service.CreateUser(ctx, CreateUserCommand{Email: "owner@example.com", Password: "password123"})
	login, _ := service.Login(ctx, LoginCommand{Email: owner.Email, Password: "password123"})
	principal, _ := service.AuthenticateSessionPrincipal(ctx, login.AccessToken)
	if err := service.RemoveWorkspaceMember(ctx, principal, owner.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("remove final owner error = %v", err)
	}
	if err := service.UpdateWorkspaceMember(ctx, principal, UpdateWorkspaceMemberCommand{UserID: owner.ID, Role: RoleAdmin}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("demote final owner error = %v", err)
	}
}

type fakeRepository struct {
	usersByID              map[string]User
	userIDByEmail          map[string]string
	workspacesByID         map[string]Workspace
	membershipsByUserID    map[string][]WorkspaceMembership
	invitationsByID        map[string]WorkspaceInvitation
	invitationIDByHash     map[string]string
	identitiesByKey        map[string]Identity
	sessionsByHash         map[string]Session
	apiKeysByID            map[string]APIKey
	apiKeyIDByHash         map[string]string
	apiKeyEvents           []APIKeyEvent
	saveSessionError       error
	apiKeyPrincipalLookups int
	lastUsedUpdates        int
	updateLastUsedError    error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		usersByID:           map[string]User{},
		userIDByEmail:       map[string]string{},
		workspacesByID:      map[string]Workspace{},
		membershipsByUserID: map[string][]WorkspaceMembership{},
		invitationsByID:     map[string]WorkspaceInvitation{},
		invitationIDByHash:  map[string]string{},
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

func (r *fakeRepository) ListWorkspaceAccessByUserID(_ context.Context, userID string) ([]WorkspaceAccess, error) {
	items := []WorkspaceAccess{}
	for _, membership := range r.membershipsByUserID[userID] {
		if workspace, ok := r.workspacesByID[membership.WorkspaceID]; ok {
			items = append(items, WorkspaceAccess{Workspace: workspace, Membership: membership})
		}
	}
	return items, nil
}

func (r *fakeRepository) FindWorkspaceAccess(_ context.Context, workspaceID string, userID string) (WorkspaceAccess, error) {
	for _, membership := range r.membershipsByUserID[userID] {
		if membership.WorkspaceID == workspaceID {
			workspace, ok := r.workspacesByID[workspaceID]
			if !ok {
				return WorkspaceAccess{}, domain.ErrNotFound
			}
			return WorkspaceAccess{Workspace: workspace, Membership: membership}, nil
		}
	}
	return WorkspaceAccess{}, domain.ErrNotFound
}

func (r *fakeRepository) ListWorkspaceMembers(_ context.Context, workspaceID string) ([]WorkspaceMember, error) {
	items := []WorkspaceMember{}
	for userID, memberships := range r.membershipsByUserID {
		for _, membership := range memberships {
			if membership.WorkspaceID == workspaceID {
				items = append(items, WorkspaceMember{WorkspaceID: workspaceID, UserID: userID, Email: r.usersByID[userID].Email, Role: membership.Role, CreatedAt: membership.CreatedAt})
			}
		}
	}
	return items, nil
}

func (r *fakeRepository) CountWorkspaceOwners(_ context.Context, workspaceID string) (int, error) {
	count := 0
	for _, memberships := range r.membershipsByUserID {
		for _, membership := range memberships {
			if membership.WorkspaceID == workspaceID && membership.Role == RoleOwner {
				count++
			}
		}
	}
	return count, nil
}

func (r *fakeRepository) UpdateWorkspaceMembershipRole(_ context.Context, workspaceID string, userID string, role string) error {
	memberships := r.membershipsByUserID[userID]
	for index := range memberships {
		if memberships[index].WorkspaceID == workspaceID {
			memberships[index].Role = role
			r.membershipsByUserID[userID] = memberships
			return nil
		}
	}
	return domain.ErrNotFound
}

func (r *fakeRepository) DeleteWorkspaceMembership(_ context.Context, workspaceID string, userID string) error {
	memberships := r.membershipsByUserID[userID]
	for index, membership := range memberships {
		if membership.WorkspaceID == workspaceID {
			r.membershipsByUserID[userID] = append(memberships[:index], memberships[index+1:]...)
			return nil
		}
	}
	return domain.ErrNotFound
}

func (r *fakeRepository) SaveWorkspaceInvitation(_ context.Context, invitation WorkspaceInvitation) (WorkspaceInvitation, error) {
	for _, existing := range r.invitationsByID {
		if existing.WorkspaceID == invitation.WorkspaceID && existing.Email == invitation.Email && existing.AcceptedAt == nil && existing.RevokedAt == nil {
			return WorkspaceInvitation{}, domain.ErrConflict
		}
	}
	r.invitationsByID[invitation.ID] = invitation
	r.invitationIDByHash[invitation.TokenHash] = invitation.ID
	return invitation, nil
}

func (r *fakeRepository) ListWorkspaceInvitations(_ context.Context, workspaceID string) ([]WorkspaceInvitation, error) {
	items := []WorkspaceInvitation{}
	for _, invitation := range r.invitationsByID {
		if invitation.WorkspaceID == workspaceID {
			items = append(items, invitation)
		}
	}
	return items, nil
}

func (r *fakeRepository) FindWorkspaceInvitationByTokenHash(_ context.Context, tokenHash string) (WorkspaceInvitation, error) {
	id, ok := r.invitationIDByHash[tokenHash]
	if !ok {
		return WorkspaceInvitation{}, domain.ErrNotFound
	}
	return r.invitationsByID[id], nil
}

func (r *fakeRepository) AcceptWorkspaceInvitation(_ context.Context, invitation WorkspaceInvitation, membership WorkspaceMembership, acceptedAt time.Time) error {
	current := r.invitationsByID[invitation.ID]
	if current.AcceptedAt != nil || current.RevokedAt != nil || !current.ExpiresAt.After(acceptedAt) {
		return domain.ErrConflict
	}
	r.membershipsByUserID[membership.UserID] = append(r.membershipsByUserID[membership.UserID], membership)
	current.AcceptedAt = &acceptedAt
	current.AcceptedByUserID = membership.UserID
	r.invitationsByID[current.ID] = current
	return nil
}

func (r *fakeRepository) RevokeWorkspaceInvitation(_ context.Context, workspaceID string, id string, revokedAt time.Time) error {
	invitation, ok := r.invitationsByID[id]
	if !ok || invitation.WorkspaceID != workspaceID || invitation.AcceptedAt != nil || invitation.RevokedAt != nil {
		return domain.ErrNotFound
	}
	invitation.RevokedAt = &revokedAt
	r.invitationsByID[id] = invitation
	return nil
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

func (r *fakeRepository) CreateAPIKey(_ context.Context, key APIKey, event APIKeyEvent) (APIKey, error) {
	r.apiKeysByID[key.ID] = key
	r.apiKeyIDByHash[key.TokenHash] = key.ID
	r.apiKeyEvents = append(r.apiKeyEvents, event)
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

func (r *fakeRepository) FindAPIKeyPrincipalByTokenHash(_ context.Context, tokenHash string) (APIKey, User, error) {
	r.apiKeyPrincipalLookups++
	id, ok := r.apiKeyIDByHash[tokenHash]
	if !ok {
		return APIKey{}, User{}, domain.ErrNotFound
	}
	key := r.apiKeysByID[id]
	user, ok := r.usersByID[key.UserID]
	if !ok {
		return APIKey{}, User{}, domain.ErrNotFound
	}
	return key, user, nil
}

func (r *fakeRepository) FindAPIKeyByID(ctx context.Context, userID string, id string) (APIKey, error) {
	workspaceID, err := RequireWorkspaceID(ctx)
	if err != nil {
		return APIKey{}, err
	}
	key, ok := r.apiKeysByID[id]
	if !ok || key.UserID != userID || key.WorkspaceID != workspaceID {
		return APIKey{}, domain.ErrNotFound
	}
	return key, nil
}

func (r *fakeRepository) UpdateAPIKeyLastUsed(_ context.Context, id string, lastUsedAt time.Time) error {
	r.lastUsedUpdates++
	if r.updateLastUsedError != nil {
		err := r.updateLastUsedError
		r.updateLastUsedError = nil
		return err
	}
	key, ok := r.apiKeysByID[id]
	if !ok {
		return domain.ErrNotFound
	}
	key.LastUsedAt = &lastUsedAt
	r.apiKeysByID[id] = key
	return nil
}

func (r *fakeRepository) RotateAPIKey(ctx context.Context, userID string, sourceID string, replacement APIKey, revokeAt time.Time, events []APIKeyEvent) (APIKey, error) {
	source, err := r.FindAPIKeyByID(ctx, userID, sourceID)
	if err != nil {
		return APIKey{}, err
	}
	if source.RevokedAt != nil {
		return APIKey{}, domain.ErrConflict
	}
	source.RevokedAt = &revokeAt
	r.apiKeysByID[sourceID] = source
	r.apiKeysByID[replacement.ID] = replacement
	r.apiKeyIDByHash[replacement.TokenHash] = replacement.ID
	r.apiKeyEvents = append(r.apiKeyEvents, events...)
	return replacement, nil
}

func (r *fakeRepository) RevokeAPIKey(ctx context.Context, userID string, id string, revokedAt time.Time, event APIKeyEvent) error {
	key, err := r.FindAPIKeyByID(ctx, userID, id)
	if err != nil {
		return err
	}
	if key.RevokedAt != nil && !key.RevokedAt.After(revokedAt) {
		return domain.ErrNotFound
	}
	key.RevokedAt = &revokedAt
	r.apiKeysByID[id] = key
	r.apiKeyEvents = append(r.apiKeyEvents, event)
	return nil
}

func (r *fakeRepository) ListAPIKeyEvents(ctx context.Context, userID string, limit int) ([]APIKeyEvent, error) {
	workspaceID, err := RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	items := []APIKeyEvent{}
	for i := len(r.apiKeyEvents) - 1; i >= 0 && len(items) < limit; i-- {
		event := r.apiKeyEvents[i]
		if event.UserID == userID && event.WorkspaceID == workspaceID {
			items = append(items, event)
		}
	}
	return items, nil
}
