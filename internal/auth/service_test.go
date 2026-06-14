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

func TestCreateUserRequiresEmptyUserStore(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	service := NewService(repo)

	if _, err := service.CreateUser(ctx, CreateUserCommand{Email: "admin@example.com", Password: "password-one"}); err != nil {
		t.Fatalf("create first user: %v", err)
	}

	_, err := service.CreateUser(ctx, CreateUserCommand{Email: "other@example.com", Password: "password-two"})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("second bootstrap error = %v, want ErrConflict", err)
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
	if login.Token == "" || login.TokenType != "Bearer" || !login.ExpiresAt.After(service.now()) {
		t.Fatalf("login = %#v", login)
	}

	authenticated, err := service.AuthenticateSession(ctx, login.Token)
	if err != nil {
		t.Fatalf("authenticate session: %v", err)
	}
	if authenticated.ID != created.ID {
		t.Fatalf("authenticated user = %#v, want %#v", authenticated, created)
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

func (r *fakeRepository) CountUsers(context.Context) (int, error) {
	return len(r.usersByID), nil
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

func (r *fakeRepository) FindSessionByTokenHash(_ context.Context, tokenHash string, now time.Time) (Session, error) {
	session, ok := r.sessionsByHash[tokenHash]
	if !ok || !session.ExpiresAt.After(now) {
		return Session{}, domain.ErrNotFound
	}
	return session, nil
}
