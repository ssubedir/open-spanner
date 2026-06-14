package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

const (
	defaultSessionTTL  = 24 * time.Hour
	defaultTokenBytes  = 32
	minPasswordRunes   = 8
	sessionTokenPrefix = "osp_"
)

type Repository interface {
	CountUsers(ctx context.Context) (int, error)
	SaveUser(ctx context.Context, user User) (User, error)
	FindUserByID(ctx context.Context, id string) (User, error)
	FindUserByEmail(ctx context.Context, email string) (User, error)
	SaveSession(ctx context.Context, session Session) (Session, error)
	FindSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) (Session, error)
	DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error
}

type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type Session struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type CreateUserCommand struct {
	Email    string
	Password string
}

type LoginCommand struct {
	Email    string
	Password string
}

type UserResult struct {
	ID        string
	Email     string
	CreatedAt time.Time
}

type LoginResult struct {
	Token     string
	TokenType string
	ExpiresAt time.Time
	User      UserResult
}

type Service struct {
	repo       Repository
	now        func() time.Time
	sessionTTL time.Duration
	tokenBytes int
}

func NewService(repo Repository) Service {
	return Service{
		repo:       repo,
		now:        time.Now,
		sessionTTL: defaultSessionTTL,
		tokenBytes: defaultTokenBytes,
	}
}

func (s Service) CreateUser(ctx context.Context, cmd CreateUserCommand) (UserResult, error) {
	email, err := normalizeEmail(cmd.Email)
	if err != nil {
		return UserResult{}, err
	}
	if err := validatePassword(cmd.Password); err != nil {
		return UserResult{}, err
	}

	count, err := s.repo.CountUsers(ctx)
	if err != nil {
		return UserResult{}, err
	}
	if count > 0 {
		return UserResult{}, errors.Join(domain.ErrConflict, errors.New("user creation requires authentication"))
	}

	passwordHash, err := hashPassword(cmd.Password)
	if err != nil {
		return UserResult{}, err
	}

	user, err := s.repo.SaveUser(ctx, User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    s.now().UTC(),
	})
	if err != nil {
		return UserResult{}, err
	}

	return userResult(user), nil
}

func (s Service) Login(ctx context.Context, cmd LoginCommand) (LoginResult, error) {
	email, err := normalizeEmail(cmd.Email)
	if err != nil {
		return LoginResult{}, unauthorized()
	}

	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return LoginResult{}, unauthorized()
		}
		return LoginResult{}, err
	}
	if err := verifyPassword(user.PasswordHash, cmd.Password); err != nil {
		return LoginResult{}, unauthorized()
	}

	token, err := newSessionToken(s.tokenBytes)
	if err != nil {
		return LoginResult{}, err
	}

	now := s.now().UTC()
	session, err := s.repo.SaveSession(ctx, Session{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: HashToken(token),
		CreatedAt: now,
		ExpiresAt: now.Add(s.sessionTTL),
	})
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		Token:     token,
		TokenType: "Bearer",
		ExpiresAt: session.ExpiresAt,
		User:      userResult(user),
	}, nil
}

func (s Service) AuthenticateSession(ctx context.Context, token string) (UserResult, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return UserResult{}, unauthorized()
	}

	session, err := s.repo.FindSessionByTokenHash(ctx, HashToken(token), s.now().UTC())
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return UserResult{}, unauthorized()
		}
		return UserResult{}, err
	}

	user, err := s.repo.FindUserByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return UserResult{}, unauthorized()
		}
		return UserResult{}, err
	}

	return userResult(user), nil
}

func (s Service) DeleteSession(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	return s.repo.DeleteSessionByTokenHash(ctx, HashToken(token))
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return "", errors.Join(domain.ErrInvalidInput, errors.New("email is required"))
	}
	return email, nil
}

func validatePassword(password string) error {
	if utf8.RuneCountInString(password) < minPasswordRunes {
		return errors.Join(domain.ErrInvalidInput, errors.New("password must be at least 8 characters"))
	}
	return nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func verifyPassword(hash string, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func newSessionToken(byteCount int) (string, error) {
	if byteCount <= 0 {
		byteCount = defaultTokenBytes
	}
	data := make([]byte, byteCount)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return sessionTokenPrefix + base64.RawURLEncoding.EncodeToString(data), nil
}

func unauthorized() error {
	return errors.Join(domain.ErrUnauthorized, errors.New("invalid credentials"))
}

func userResult(user User) UserResult {
	return UserResult{
		ID:        user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
}
