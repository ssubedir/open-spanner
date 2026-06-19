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
	defaultAccessTokenTTL  = 15 * time.Minute
	defaultRefreshTokenTTL = 30 * 24 * time.Hour
	defaultTokenBytes      = 32
	minPasswordRunes       = 8
	accessTokenPrefix      = "osp_at_"
	refreshTokenPrefix     = "osp_rt_"
	apiKeyPrefix           = "osp_sk_"
	TokenKindAccess        = "access"
	TokenKindRefresh       = "refresh"
)

type Repository interface {
	SaveUser(ctx context.Context, user User) (User, error)
	FindUserByID(ctx context.Context, id string) (User, error)
	FindUserByEmail(ctx context.Context, email string) (User, error)
	SaveSession(ctx context.Context, session Session) (Session, error)
	FindSessionByTokenHash(ctx context.Context, tokenHash string, kind string, now time.Time) (Session, error)
	DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error
	SaveAPIKey(ctx context.Context, key APIKey) (APIKey, error)
	ListAPIKeys(ctx context.Context, userID string) ([]APIKey, error)
	FindAPIKeyByTokenHash(ctx context.Context, tokenHash string) (APIKey, error)
	UpdateAPIKeyLastUsed(ctx context.Context, id string, lastUsedAt time.Time) error
	DeleteAPIKey(ctx context.Context, userID string, id string) error
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
	Kind      string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type APIKey struct {
	ID            string
	UserID        string
	Name          string
	TokenHash     string
	Prefix        string
	Scopes        []string
	AllowedMeters []string
	ExpiresAt     *time.Time
	RevokedAt     *time.Time
	CreatedAt     time.Time
	LastUsedAt    *time.Time
}

type CreateUserCommand struct {
	Email    string
	Password string
}

type LoginCommand struct {
	Email    string
	Password string
}

type CreateAPIKeyCommand struct {
	UserID        string
	Name          string
	Scopes        []string
	AllowedMeters []string
	ExpiresAt     *time.Time
}

type UserResult struct {
	ID        string
	Email     string
	CreatedAt time.Time
}

type APIKeyResult struct {
	ID            string
	Name          string
	Prefix        string
	Scopes        []string
	AllowedMeters []string
	ExpiresAt     *time.Time
	RevokedAt     *time.Time
	CreatedAt     time.Time
	LastUsedAt    *time.Time
}

type CreateAPIKeyResult struct {
	APIKeyResult
	Key string
}

type LoginResult struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	TokenType        string
	User             UserResult
}

type RefreshResult struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	TokenType        string
	User             UserResult
}

type Service struct {
	repo            Repository
	now             func() time.Time
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	tokenBytes      int
}

func NewService(repo Repository) Service {
	return Service{
		repo:            repo,
		now:             time.Now,
		accessTokenTTL:  defaultAccessTokenTTL,
		refreshTokenTTL: defaultRefreshTokenTTL,
		tokenBytes:      defaultTokenBytes,
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

func (s Service) CreateAPIKey(ctx context.Context, cmd CreateAPIKeyCommand) (CreateAPIKeyResult, error) {
	userID := strings.TrimSpace(cmd.UserID)
	if userID == "" {
		return CreateAPIKeyResult{}, errors.Join(domain.ErrInvalidInput, errors.New("user id is required"))
	}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return CreateAPIKeyResult{}, errors.Join(domain.ErrInvalidInput, errors.New("name is required"))
	}
	scopes, err := normalizeAPIKeyScopes(cmd.Scopes)
	if err != nil {
		return CreateAPIKeyResult{}, err
	}
	allowedMeters := normalizeAllowedMeters(cmd.AllowedMeters)
	expiresAt := cmd.ExpiresAt
	if expiresAt != nil && !expiresAt.After(s.now().UTC()) {
		return CreateAPIKeyResult{}, errors.Join(domain.ErrInvalidInput, errors.New("expires_at must be in the future"))
	}

	token, err := newSessionToken(apiKeyPrefix, s.tokenBytes)
	if err != nil {
		return CreateAPIKeyResult{}, err
	}

	key, err := s.repo.SaveAPIKey(ctx, APIKey{
		ID:            uuid.NewString(),
		UserID:        userID,
		Name:          name,
		TokenHash:     HashToken(token),
		Prefix:        tokenPrefix(token),
		Scopes:        scopes,
		AllowedMeters: allowedMeters,
		ExpiresAt:     expiresAt,
		CreatedAt:     s.now().UTC(),
	})
	if err != nil {
		return CreateAPIKeyResult{}, err
	}

	result := apiKeyResult(key)
	return CreateAPIKeyResult{
		APIKeyResult: result,
		Key:          token,
	}, nil
}

func (s Service) ListAPIKeys(ctx context.Context, userID string) ([]APIKeyResult, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.Join(domain.ErrInvalidInput, errors.New("user id is required"))
	}

	keys, err := s.repo.ListAPIKeys(ctx, userID)
	if err != nil {
		return nil, err
	}

	results := make([]APIKeyResult, 0, len(keys))
	for _, key := range keys {
		results = append(results, apiKeyResult(key))
	}
	return results, nil
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

	accessToken, err := newSessionToken(accessTokenPrefix, s.tokenBytes)
	if err != nil {
		return LoginResult{}, err
	}
	refreshToken, err := newSessionToken(refreshTokenPrefix, s.tokenBytes)
	if err != nil {
		return LoginResult{}, err
	}

	now := s.now().UTC()
	accessSession, err := s.repo.SaveSession(ctx, Session{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: HashToken(accessToken),
		Kind:      TokenKindAccess,
		CreatedAt: now,
		ExpiresAt: now.Add(s.accessTokenTTL),
	})
	if err != nil {
		return LoginResult{}, err
	}
	refreshSession, err := s.repo.SaveSession(ctx, Session{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: HashToken(refreshToken),
		Kind:      TokenKindRefresh,
		CreatedAt: now,
		ExpiresAt: now.Add(s.refreshTokenTTL),
	})
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		AccessToken:      accessToken,
		AccessExpiresAt:  accessSession.ExpiresAt,
		RefreshToken:     refreshToken,
		RefreshExpiresAt: refreshSession.ExpiresAt,
		TokenType:        "Bearer",
		User:             userResult(user),
	}, nil
}

func (s Service) AuthenticateSession(ctx context.Context, token string) (UserResult, error) {
	return s.authenticateToken(ctx, token, TokenKindAccess)
}

func (s Service) AuthenticateAPIKey(ctx context.Context, token string) (UserResult, error) {
	principal, err := s.AuthenticateAPIKeyPrincipal(ctx, token)
	if err != nil {
		return UserResult{}, err
	}
	return principal.User, nil
}

func (s Service) AuthenticateSessionPrincipal(ctx context.Context, token string) (Principal, error) {
	user, err := s.AuthenticateSession(ctx, token)
	if err != nil {
		return Principal{}, err
	}
	return Principal{
		Kind: PrincipalKindSession,
		ID:   user.ID,
		User: user,
	}, nil
}

func (s Service) AuthenticateAPIKeyPrincipal(ctx context.Context, token string) (Principal, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Principal{}, unauthorized()
	}

	key, err := s.repo.FindAPIKeyByTokenHash(ctx, HashToken(token))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return Principal{}, unauthorized()
		}
		return Principal{}, err
	}

	now := s.now().UTC()
	if key.RevokedAt != nil {
		return Principal{}, unauthorized()
	}
	if key.ExpiresAt != nil && !key.ExpiresAt.After(now) {
		return Principal{}, unauthorized()
	}
	if err := s.repo.UpdateAPIKeyLastUsed(ctx, key.ID, now); err != nil {
		return Principal{}, err
	}

	user, err := s.repo.FindUserByID(ctx, key.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return Principal{}, unauthorized()
		}
		return Principal{}, err
	}

	result := userResult(user)
	return Principal{
		Kind:          PrincipalKindAPIKey,
		ID:            key.ID,
		User:          result,
		APIKeyID:      key.ID,
		Scopes:        append([]string(nil), key.Scopes...),
		AllowedMeters: append([]string(nil), key.AllowedMeters...),
		ExpiresAt:     key.ExpiresAt,
		RevokedAt:     key.RevokedAt,
	}, nil
}

func (s Service) RefreshSession(ctx context.Context, token string) (RefreshResult, error) {
	user, err := s.authenticateToken(ctx, token, TokenKindRefresh)
	if err != nil {
		return RefreshResult{}, err
	}

	if err := s.DeleteSession(ctx, token); err != nil {
		return RefreshResult{}, err
	}

	accessToken, err := newSessionToken(accessTokenPrefix, s.tokenBytes)
	if err != nil {
		return RefreshResult{}, err
	}
	refreshToken, err := newSessionToken(refreshTokenPrefix, s.tokenBytes)
	if err != nil {
		return RefreshResult{}, err
	}

	now := s.now().UTC()
	accessSession, err := s.repo.SaveSession(ctx, Session{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: HashToken(accessToken),
		Kind:      TokenKindAccess,
		CreatedAt: now,
		ExpiresAt: now.Add(s.accessTokenTTL),
	})
	if err != nil {
		return RefreshResult{}, err
	}
	refreshSession, err := s.repo.SaveSession(ctx, Session{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: HashToken(refreshToken),
		Kind:      TokenKindRefresh,
		CreatedAt: now,
		ExpiresAt: now.Add(s.refreshTokenTTL),
	})
	if err != nil {
		return RefreshResult{}, err
	}

	return RefreshResult{
		AccessToken:      accessToken,
		AccessExpiresAt:  accessSession.ExpiresAt,
		RefreshToken:     refreshToken,
		RefreshExpiresAt: refreshSession.ExpiresAt,
		TokenType:        "Bearer",
		User:             user,
	}, nil
}

func (s Service) authenticateToken(ctx context.Context, token string, kind string) (UserResult, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return UserResult{}, unauthorized()
	}

	session, err := s.repo.FindSessionByTokenHash(ctx, HashToken(token), kind, s.now().UTC())
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

func (s Service) DeleteAPIKey(ctx context.Context, userID string, id string) error {
	userID = strings.TrimSpace(userID)
	id = strings.TrimSpace(id)
	if userID == "" || id == "" {
		return errors.Join(domain.ErrInvalidInput, errors.New("api key id is required"))
	}
	return s.repo.DeleteAPIKey(ctx, userID, id)
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

func newSessionToken(prefix string, byteCount int) (string, error) {
	if byteCount <= 0 {
		byteCount = defaultTokenBytes
	}
	data := make([]byte, byteCount)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(data), nil
}

func tokenPrefix(token string) string {
	if len(token) <= 18 {
		return token
	}
	return token[:18]
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

func apiKeyResult(key APIKey) APIKeyResult {
	return APIKeyResult{
		ID:            key.ID,
		Name:          key.Name,
		Prefix:        key.Prefix,
		Scopes:        append([]string(nil), key.Scopes...),
		AllowedMeters: append([]string(nil), key.AllowedMeters...),
		ExpiresAt:     key.ExpiresAt,
		RevokedAt:     key.RevokedAt,
		CreatedAt:     key.CreatedAt,
		LastUsedAt:    key.LastUsedAt,
	}
}
