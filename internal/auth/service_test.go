package auth

import (
	"context"
	"errors"
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

type fakeRepository struct {
	usersByID        map[string]User
	userIDByEmail    map[string]string
	sessionsByHash   map[string]Session
	saveSessionError error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		usersByID:      map[string]User{},
		userIDByEmail:  map[string]string{},
		sessionsByHash: map[string]Session{},
	}
}

func (r *fakeRepository) SaveUser(_ context.Context, user User) (User, error) {
	r.usersByID[user.ID] = user
	r.userIDByEmail[user.Email] = user.ID
	return user, nil
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
