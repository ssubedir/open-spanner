package savedquery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

const (
	DefaultLimit = 500
	MaxNameRunes = 120
)

type Repository interface {
	Save(ctx context.Context, query SavedQuery) (SavedQuery, error)
	Find(ctx context.Context, query FindQuery) ([]SavedQuery, error)
	Delete(ctx context.Context, userID string, id string) error
}

type Service interface {
	Create(ctx context.Context, cmd SaveCommand) (Result, error)
	List(ctx context.Context, query ListQuery) (ListResult, error)
	Update(ctx context.Context, cmd UpdateCommand) (Result, error)
	Delete(ctx context.Context, cmd DeleteCommand) error
}

type SavedQuery struct {
	ID         string
	UserID     string
	Name       string
	Query      json.RawMessage
	GroupBy    []string
	BucketSize string
	Limit      int
	Pinned     bool
	Position   int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type FindQuery struct {
	UserID string
	ID     string
}

type SaveCommand struct {
	UserID     string
	Name       string
	Query      json.RawMessage
	GroupBy    []string
	BucketSize string
	Limit      int
	Pinned     bool
	Position   int
}

type UpdateCommand struct {
	ID         string
	UserID     string
	Name       string
	Query      json.RawMessage
	GroupBy    []string
	BucketSize string
	Limit      int
	Pinned     *bool
	Position   *int
}

type DeleteCommand struct {
	ID     string
	UserID string
}

type ListQuery struct {
	UserID string
}

type Result struct {
	ID         string
	Name       string
	Query      json.RawMessage
	GroupBy    []string
	BucketSize string
	Limit      int
	Pinned     bool
	Position   int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ListResult struct {
	Items []Result
}

type service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) Service {
	return &service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *service) Create(ctx context.Context, cmd SaveCommand) (Result, error) {
	query, err := savedQueryFromInput("", cmd.UserID, cmd.Name, cmd.Query, cmd.GroupBy, cmd.BucketSize, cmd.Limit, s.now().UTC())
	if err != nil {
		return Result{}, err
	}

	query.ID = uuid.NewString()
	query.Pinned = cmd.Pinned
	query.Position = normalizePosition(cmd.Position)
	saved, err := s.repo.Save(ctx, query)
	if err != nil {
		return Result{}, err
	}
	return resultFromSavedQuery(saved), nil
}

func (s *service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	userID, err := normalizeRequired(query.UserID, "user id is required")
	if err != nil {
		return ListResult{}, err
	}

	queries, err := s.repo.Find(ctx, FindQuery{UserID: userID})
	if err != nil {
		return ListResult{}, err
	}

	results := make([]Result, 0, len(queries))
	for _, query := range queries {
		results = append(results, resultFromSavedQuery(query))
	}
	return ListResult{Items: results}, nil
}

func (s *service) Update(ctx context.Context, cmd UpdateCommand) (Result, error) {
	id, err := normalizeRequired(cmd.ID, "saved query id is required")
	if err != nil {
		return Result{}, err
	}
	userID, err := normalizeRequired(cmd.UserID, "user id is required")
	if err != nil {
		return Result{}, err
	}

	existing, err := s.repo.Find(ctx, FindQuery{UserID: userID, ID: id})
	if err != nil {
		return Result{}, err
	}
	if len(existing) == 0 {
		return Result{}, domain.ErrNotFound
	}

	next, err := savedQueryFromInput(id, userID, cmd.Name, cmd.Query, cmd.GroupBy, cmd.BucketSize, cmd.Limit, existing[0].CreatedAt)
	if err != nil {
		return Result{}, err
	}
	next.Pinned = existing[0].Pinned
	if cmd.Pinned != nil {
		next.Pinned = *cmd.Pinned
	}
	next.Position = existing[0].Position
	if cmd.Position != nil {
		next.Position = normalizePosition(*cmd.Position)
	}
	next.UpdatedAt = s.now().UTC()

	saved, err := s.repo.Save(ctx, next)
	if err != nil {
		return Result{}, err
	}
	return resultFromSavedQuery(saved), nil
}

func (s *service) Delete(ctx context.Context, cmd DeleteCommand) error {
	id, err := normalizeRequired(cmd.ID, "saved query id is required")
	if err != nil {
		return err
	}
	userID, err := normalizeRequired(cmd.UserID, "user id is required")
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, userID, id)
}

func savedQueryFromInput(id string, userID string, name string, query json.RawMessage, groupBy []string, bucketSize string, limit int, createdAt time.Time) (SavedQuery, error) {
	userID, err := normalizeRequired(userID, "user id is required")
	if err != nil {
		return SavedQuery{}, err
	}
	name, err = normalizeName(name)
	if err != nil {
		return SavedQuery{}, err
	}
	query, err = normalizeQuery(query)
	if err != nil {
		return SavedQuery{}, err
	}
	groupBy, err = domainusage.NormalizeGroupBy(groupBy)
	if err != nil {
		return SavedQuery{}, err
	}
	bucketSize, err = normalizeBucketSize(bucketSize)
	if err != nil {
		return SavedQuery{}, err
	}

	now := time.Now().UTC()
	if createdAt.IsZero() {
		createdAt = now
	}
	return SavedQuery{
		ID:         strings.TrimSpace(id),
		UserID:     userID,
		Name:       name,
		Query:      query,
		GroupBy:    groupBy,
		BucketSize: bucketSize,
		Limit:      normalizeLimit(limit),
		Position:   0,
		CreatedAt:  createdAt.UTC(),
		UpdatedAt:  createdAt.UTC(),
	}, nil
}

func normalizeRequired(value string, message string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.Join(domain.ErrInvalidInput, errors.New(message))
	}
	return value, nil
}

func normalizeName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.Join(domain.ErrInvalidInput, errors.New("name is required"))
	}
	if utf8.RuneCountInString(value) > MaxNameRunes {
		return "", errors.Join(domain.ErrInvalidInput, errors.New("name is too long"))
	}
	return value, nil
}

func normalizeQuery(value json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(value)) == 0 {
		return nil, errors.Join(domain.ErrInvalidInput, errors.New("query is required"))
	}

	var decoded map[string]any
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return nil, errors.Join(domain.ErrInvalidInput, err)
	}
	if len(decoded) == 0 {
		return nil, errors.Join(domain.ErrInvalidInput, errors.New("query is required"))
	}

	encoded, err := json.Marshal(decoded)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}

func normalizeBucketSize(value string) (string, error) {
	switch domainusage.BucketSize(strings.TrimSpace(value)) {
	case "", domainusage.BucketDay:
		return string(domainusage.BucketDay), nil
	case domainusage.BucketHour:
		return string(domainusage.BucketHour), nil
	case domainusage.BucketMonth:
		return string(domainusage.BucketMonth), nil
	default:
		return "", errors.Join(domain.ErrInvalidInput, errors.New("unsupported bucket size"))
	}
}

func normalizeLimit(value int) int {
	if value <= 0 {
		return DefaultLimit
	}
	if value > domainusage.MaxLimit {
		return domainusage.MaxLimit
	}
	return value
}

func normalizePosition(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func resultFromSavedQuery(query SavedQuery) Result {
	groupBy := make([]string, len(query.GroupBy))
	copy(groupBy, query.GroupBy)
	return Result{
		ID:         query.ID,
		Name:       query.Name,
		Query:      append(json.RawMessage(nil), query.Query...),
		GroupBy:    groupBy,
		BucketSize: query.BucketSize,
		Limit:      query.Limit,
		Pinned:     query.Pinned,
		Position:   query.Position,
		CreatedAt:  query.CreatedAt,
		UpdatedAt:  query.UpdatedAt,
	}
}
