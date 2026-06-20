package alert

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/ssubedir/open-spanner/internal/metering/app/page"
	apptransaction "github.com/ssubedir/open-spanner/internal/metering/app/transaction"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

const (
	DefaultWindow             = time.Hour
	DefaultEvaluationInterval = time.Minute
	DefaultEvaluationDelay    = 5 * time.Second
	DefaultJobLimit           = 100
	MaxNameRunes              = 120
	WebhookSecretPrefix       = "osp_whsec_"
)

type Comparator string

const (
	ComparatorGreaterThan        Comparator = "gt"
	ComparatorGreaterThanOrEqual Comparator = "gte"
	ComparatorLessThan           Comparator = "lt"
	ComparatorLessThanOrEqual    Comparator = "lte"
	ComparatorEqual              Comparator = "eq"
	ComparatorNotEqual           Comparator = "neq"
)

type StateStatus string

const (
	StateOK       StateStatus = "ok"
	StateAlerting StateStatus = "alerting"
	StateNoData   StateStatus = "no_data"
	StateError    StateStatus = "error"
)

type EventType string

const (
	EventTriggered        EventType = "triggered"
	EventResolved         EventType = "resolved"
	EventEvaluationFailed EventType = "evaluation_failed"
)

type TriggerType string

const (
	TriggerWebhook TriggerType = "webhook"
)

type DestinationType string

const (
	DestinationWebhook DestinationType = "webhook"
)

const (
	WebhookSignatureAlgorithm = "hmac-sha256"
	WebhookSignatureHeader    = "X-Open-Spanner-Signature"
	WebhookTimestampHeader    = "X-Open-Spanner-Timestamp"
	WebhookSignatureVersion   = "v1"
)

type DeliveryStatus string

const (
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryFailed    DeliveryStatus = "failed"
)

type Repository interface {
	SaveDestination(ctx context.Context, destination Destination) (Destination, error)
	FindDestinations(ctx context.Context, query DestinationQuery) ([]Destination, error)
	DeleteDestination(ctx context.Context, id string) error
	SaveRule(ctx context.Context, rule Rule) (Rule, error)
	FindRules(ctx context.Context, query RuleQuery) ([]Rule, error)
	DeleteRule(ctx context.Context, id string) error
	SaveState(ctx context.Context, state State) (State, error)
	FindState(ctx context.Context, ruleID string, groupKey string, groupValue string) (State, bool, error)
	FindStates(ctx context.Context, ruleID string, limit int) ([]State, error)
	SaveEvent(ctx context.Context, event Event) (Event, error)
	FindEvents(ctx context.Context, query EventQuery) ([]Event, error)
	SaveDelivery(ctx context.Context, delivery Delivery) (Delivery, error)
	EnqueueEvaluationJob(ctx context.Context, ruleID string, runAfter time.Time, now time.Time) error
	EnqueueDueEvaluationJobs(ctx context.Context, now time.Time, limit int) (int, error)
	ClaimEvaluationJob(ctx context.Context, now time.Time, lockedUntil time.Time, maxAttempts int) (EvaluationJob, error)
	CompleteEvaluationJob(ctx context.Context, ruleID string) error
	RequeueEvaluationJob(ctx context.Context, ruleID string, runAfter time.Time, now time.Time) error
	UpdateRuleNextEvaluation(ctx context.Context, id string, nextEvaluateAt time.Time, updatedAt time.Time) error
}

type UsageRepository interface {
	Aggregate(ctx context.Context, query domainusage.AggregateQuery) (domainusage.Aggregate, error)
	FindBreakdown(ctx context.Context, query domainusage.BreakdownQuery) ([]domainusage.BreakdownItem, error)
}

type Service interface {
	CreateDestination(ctx context.Context, cmd DestinationSaveCommand) (DestinationResult, error)
	UpdateDestination(ctx context.Context, cmd DestinationUpdateCommand) (DestinationResult, error)
	DeleteDestination(ctx context.Context, cmd DestinationDeleteCommand) error
	ListDestinations(ctx context.Context, query DestinationListQuery) (DestinationListResult, error)
	RotateDestinationSecret(ctx context.Context, cmd RotateDestinationSecretCommand) (DestinationResult, error)
	Create(ctx context.Context, cmd SaveCommand) (RuleResult, error)
	Update(ctx context.Context, cmd UpdateCommand) (RuleResult, error)
	Delete(ctx context.Context, cmd DeleteCommand) error
	Get(ctx context.Context, query GetQuery) (RuleResult, error)
	List(ctx context.Context, query ListQuery) (RuleListResult, error)
	ListEvents(ctx context.Context, query EventListQuery) (EventListResult, error)
	EnqueueForUsageEvents(ctx context.Context, events []UsageEvent) error
	EnqueueDueRules(ctx context.Context, limit int) (int, error)
	ClaimEvaluationJob(ctx context.Context, cmd ClaimCommand) (EvaluationJobResult, bool, error)
	CompleteEvaluationJob(ctx context.Context, cmd CompleteCommand) error
	FailEvaluationJob(ctx context.Context, cmd FailCommand) error
	RecordDelivery(ctx context.Context, cmd DeliveryCommand) (DeliveryResult, error)
	Evaluate(ctx context.Context, cmd EvaluateCommand) (EvaluationResult, error)
}

type service struct {
	repo       Repository
	meterRepo  domainmeter.Repository
	usageRepo  UsageRepository
	transactor apptransaction.Transactor
	now        func() time.Time
}

func NewService(repo Repository, meterRepo domainmeter.Repository, usageRepo UsageRepository, transactor apptransaction.Transactor) Service {
	if transactor == nil {
		panic("alert service requires a transactor")
	}

	return &service{
		repo:       repo,
		meterRepo:  meterRepo,
		usageRepo:  usageRepo,
		transactor: transactor,
		now:        func() time.Time { return time.Now().UTC() },
	}
}

type Rule struct {
	ID                 string
	Name               string
	MeterName          string
	Enabled            bool
	Subject            string
	Metadata           map[string]string
	Window             time.Duration
	Comparator         Comparator
	Threshold          float64
	EvaluationInterval time.Duration
	GroupBy            string
	DestinationID      string
	NextEvaluateAt     time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Destination struct {
	ID            string
	Name          string
	Type          DestinationType
	Enabled       bool
	WebhookURL    string
	WebhookSecret string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type State struct {
	RuleID      string
	GroupKey    string
	GroupValue  string
	Status      StateStatus
	Value       float64
	Message     string
	EvaluatedAt time.Time
	UpdatedAt   time.Time
}

type Event struct {
	ID         string
	RuleID     string
	GroupKey   string
	GroupValue string
	Type       EventType
	Value      float64
	Message    string
	CreatedAt  time.Time
	Delivery   *Delivery
}

type Delivery struct {
	ID          string
	EventID     string
	TriggerType TriggerType
	Status      DeliveryStatus
	StatusCode  int
	Error       string
	Duration    time.Duration
	AttemptedAt time.Time
	CreatedAt   time.Time
}

type EvaluationJob struct {
	WorkspaceID string
	RuleID      string
	RunAfter    time.Time
	LockedUntil time.Time
	Attempts    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type RuleQuery struct {
	ID            string
	MeterName     string
	DestinationID string
	Enabled       *bool
	Limit         int
}

type DestinationQuery struct {
	ID      string
	Type    string
	Enabled *bool
	Limit   int
}

type EventQuery struct {
	RuleID    string
	Limit     int
	CreatedAt time.Time
	ID        string
}

type UsageEvent struct {
	Subject  string
	Meter    string
	Metadata map[string]any
}

type SaveCommand struct {
	Name               string
	MeterName          string
	Enabled            *bool
	Subject            string
	Metadata           map[string]string
	Window             time.Duration
	Comparator         string
	Threshold          float64
	EvaluationInterval time.Duration
	GroupBy            string
	DestinationID      string
}

type UpdateCommand struct {
	ID                 string
	Name               string
	MeterName          string
	Enabled            *bool
	Subject            *string
	Metadata           *map[string]string
	Window             time.Duration
	Comparator         string
	Threshold          *float64
	EvaluationInterval time.Duration
	GroupBy            *string
	DestinationID      *string
}

type DestinationSaveCommand struct {
	Name       string
	Type       string
	Enabled    *bool
	WebhookURL string
}

type DestinationUpdateCommand struct {
	ID         string
	Name       string
	Type       string
	Enabled    *bool
	WebhookURL *string
}

type DestinationDeleteCommand struct {
	ID string
}

type RotateDestinationSecretCommand struct {
	ID string
}

type DeleteCommand struct {
	ID string
}

type GetQuery struct {
	ID string
}

type ListQuery struct {
	MeterName     string
	DestinationID string
	Enabled       *bool
	Limit         int
}

type DestinationListQuery struct {
	Type    string
	Enabled *bool
	Limit   int
}

type EventListQuery struct {
	RuleID string
	Limit  int
	Cursor string
}

type ClaimCommand struct {
	LockTTL     time.Duration
	MaxAttempts int
}

type CompleteCommand struct {
	RuleID string
}

type FailCommand struct {
	RuleID      string
	RetryAfter  time.Duration
	MaxAttempts int
	Error       string
}

type EvaluateCommand struct {
	RuleID string
}

type DeliveryCommand struct {
	EventID     string
	TriggerType string
	Status      string
	StatusCode  int
	Error       string
	Duration    time.Duration
	AttemptedAt time.Time
}

type RuleResult struct {
	ID                 string
	Name               string
	MeterName          string
	Enabled            bool
	Subject            string
	Metadata           map[string]string
	WindowSeconds      int
	Comparator         string
	Threshold          float64
	EvaluationInterval int
	GroupBy            string
	DestinationID      string
	Destination        *DestinationResult
	NextEvaluateAt     time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	State              *StateResult
	States             []StateResult
}

type DestinationResult struct {
	ID              string
	Name            string
	Type            string
	Enabled         bool
	WebhookURL      string
	WebhookSecret   string
	SignatureHeader string
	TimestampHeader string
	Algorithm       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type DestinationListResult struct {
	Items []DestinationResult
}

type StateResult struct {
	Status      string
	GroupKey    string
	GroupValue  string
	Value       float64
	Message     string
	EvaluatedAt time.Time
	UpdatedAt   time.Time
}

type RuleListResult struct {
	Items []RuleResult
}

type EventResult struct {
	ID         string
	RuleID     string
	GroupKey   string
	GroupValue string
	Type       string
	Value      float64
	Message    string
	CreatedAt  time.Time
	Delivery   *DeliveryResult
}

type DeliveryResult struct {
	ID          string
	EventID     string
	TriggerType string
	Status      string
	StatusCode  int
	Error       string
	DurationMs  int
	AttemptedAt time.Time
	CreatedAt   time.Time
}

type EventListResult struct {
	Items      []EventResult
	NextCursor string
}

type EvaluationJobResult struct {
	WorkspaceID string
	RuleID      string
	RunAfter    time.Time
	LockedUntil time.Time
	Attempts    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type EvaluationResult struct {
	Rule   RuleResult
	State  StateResult
	Event  *EventResult
	Events []EventResult
}

func (s *service) CreateDestination(ctx context.Context, cmd DestinationSaveCommand) (DestinationResult, error) {
	now := s.now()
	destination, err := destinationFromInput(Destination{}, cmd, now)
	if err != nil {
		return DestinationResult{}, err
	}
	destination.ID = uuid.NewString()
	destination.CreatedAt = now
	destination.UpdatedAt = now

	saved, err := s.repo.SaveDestination(ctx, destination)
	if err != nil {
		return DestinationResult{}, err
	}
	return destinationResult(saved), nil
}

func (s *service) UpdateDestination(ctx context.Context, cmd DestinationUpdateCommand) (DestinationResult, error) {
	existing, err := s.findDestination(ctx, cmd.ID)
	if err != nil {
		return DestinationResult{}, err
	}
	now := s.now()
	next := existing
	if cmd.Name != "" {
		next.Name = cmd.Name
	}
	if cmd.Type != "" {
		next.Type = DestinationType(cmd.Type)
	}
	if cmd.Enabled != nil {
		next.Enabled = *cmd.Enabled
	}
	if cmd.WebhookURL != nil {
		next.WebhookURL = *cmd.WebhookURL
	}
	destination, err := validateDestination(next, now)
	if err != nil {
		return DestinationResult{}, err
	}
	destination.CreatedAt = existing.CreatedAt
	destination.UpdatedAt = now

	saved, err := s.repo.SaveDestination(ctx, destination)
	if err != nil {
		return DestinationResult{}, err
	}
	return destinationResult(saved), nil
}

func (s *service) DeleteDestination(ctx context.Context, cmd DestinationDeleteCommand) error {
	id, err := normalizeRequired(cmd.ID, "alert destination id is required")
	if err != nil {
		return err
	}
	rules, err := s.repo.FindRules(ctx, RuleQuery{DestinationID: id, Limit: 1})
	if err != nil {
		return err
	}
	if len(rules) > 0 {
		return errors.Join(domain.ErrConflict, errors.New("alert destination is used by alert rules"))
	}
	return s.repo.DeleteDestination(ctx, id)
}

func (s *service) ListDestinations(ctx context.Context, query DestinationListQuery) (DestinationListResult, error) {
	destinations, err := s.repo.FindDestinations(ctx, DestinationQuery{
		Type:    query.Type,
		Enabled: query.Enabled,
		Limit:   normalizeLimit(query.Limit),
	})
	if err != nil {
		return DestinationListResult{}, err
	}
	results := make([]DestinationResult, 0, len(destinations))
	for _, destination := range destinations {
		results = append(results, destinationResult(destination))
	}
	return DestinationListResult{Items: results}, nil
}

func (s *service) RotateDestinationSecret(ctx context.Context, cmd RotateDestinationSecretCommand) (DestinationResult, error) {
	destination, err := s.findDestination(ctx, cmd.ID)
	if err != nil {
		return DestinationResult{}, err
	}
	if destination.Type != DestinationWebhook {
		return DestinationResult{}, errors.Join(domain.ErrInvalidInput, errors.New("only webhook destinations can rotate signing secrets"))
	}
	if destination.WebhookURL == "" {
		return DestinationResult{}, errors.Join(domain.ErrInvalidInput, errors.New("webhook url must be configured before rotating signing secrets"))
	}
	secret, err := newWebhookSecret()
	if err != nil {
		return DestinationResult{}, err
	}
	destination.WebhookSecret = secret
	destination.UpdatedAt = s.now()

	saved, err := s.repo.SaveDestination(ctx, destination)
	if err != nil {
		return DestinationResult{}, err
	}
	return destinationResult(saved), nil
}

func (s *service) Create(ctx context.Context, cmd SaveCommand) (RuleResult, error) {
	if _, err := s.findMeter(ctx, cmd.MeterName); err != nil {
		return RuleResult{}, err
	}
	if err := s.validateRequiredDestinationReference(ctx, cmd.DestinationID); err != nil {
		return RuleResult{}, err
	}

	now := s.now()
	rule, err := ruleFromInput(Rule{}, SaveCommand{
		Name:               cmd.Name,
		MeterName:          cmd.MeterName,
		Enabled:            cmd.Enabled,
		Subject:            cmd.Subject,
		Metadata:           cmd.Metadata,
		Window:             cmd.Window,
		Comparator:         cmd.Comparator,
		Threshold:          cmd.Threshold,
		EvaluationInterval: cmd.EvaluationInterval,
		GroupBy:            cmd.GroupBy,
		DestinationID:      cmd.DestinationID,
	}, now)
	if err != nil {
		return RuleResult{}, err
	}
	rule.ID = uuid.NewString()
	rule.CreatedAt = now
	rule.UpdatedAt = now
	rule.NextEvaluateAt = now

	saved, err := s.repo.SaveRule(ctx, rule)
	if err != nil {
		return RuleResult{}, err
	}
	if saved.Enabled {
		if err := s.repo.EnqueueEvaluationJob(ctx, saved.ID, now, now); err != nil {
			return RuleResult{}, err
		}
	}

	return s.ruleResult(ctx, saved), nil
}

func (s *service) Update(ctx context.Context, cmd UpdateCommand) (RuleResult, error) {
	existing, err := s.findRule(ctx, cmd.ID)
	if err != nil {
		return RuleResult{}, err
	}
	if cmd.MeterName != "" {
		if _, err := s.findMeter(ctx, cmd.MeterName); err != nil {
			return RuleResult{}, err
		}
	}
	if cmd.DestinationID != nil {
		if err := s.validateRequiredDestinationReference(ctx, *cmd.DestinationID); err != nil {
			return RuleResult{}, err
		}
	}

	now := s.now()
	next := existing
	if cmd.Name != "" {
		next.Name = cmd.Name
	}
	if cmd.MeterName != "" {
		next.MeterName = cmd.MeterName
	}
	if cmd.Enabled != nil {
		next.Enabled = *cmd.Enabled
	}
	if cmd.Subject != nil {
		next.Subject = *cmd.Subject
	}
	if cmd.Metadata != nil {
		next.Metadata = *cmd.Metadata
	}
	if cmd.Window > 0 {
		next.Window = cmd.Window
	}
	if cmd.Comparator != "" {
		next.Comparator = Comparator(cmd.Comparator)
	}
	if cmd.Threshold != nil {
		next.Threshold = *cmd.Threshold
	}
	if cmd.EvaluationInterval > 0 {
		next.EvaluationInterval = cmd.EvaluationInterval
	}
	if cmd.GroupBy != nil {
		next.GroupBy = *cmd.GroupBy
	}
	if cmd.DestinationID != nil {
		next.DestinationID = *cmd.DestinationID
	}

	rule, err := validateRule(next, now)
	if err != nil {
		return RuleResult{}, err
	}
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = now
	rule.NextEvaluateAt = now

	saved, err := s.repo.SaveRule(ctx, rule)
	if err != nil {
		return RuleResult{}, err
	}
	if saved.Enabled {
		if err := s.repo.EnqueueEvaluationJob(ctx, saved.ID, now, now); err != nil {
			return RuleResult{}, err
		}
	}

	return s.ruleResult(ctx, saved), nil
}

func (s *service) Delete(ctx context.Context, cmd DeleteCommand) error {
	id, err := normalizeRequired(cmd.ID, "alert id is required")
	if err != nil {
		return err
	}
	return s.repo.DeleteRule(ctx, id)
}

func (s *service) Get(ctx context.Context, query GetQuery) (RuleResult, error) {
	rule, err := s.findRule(ctx, query.ID)
	if err != nil {
		return RuleResult{}, err
	}
	return s.ruleResult(ctx, rule), nil
}

func (s *service) List(ctx context.Context, query ListQuery) (RuleListResult, error) {
	rules, err := s.repo.FindRules(ctx, RuleQuery{
		MeterName:     query.MeterName,
		DestinationID: query.DestinationID,
		Enabled:       query.Enabled,
		Limit:         normalizeLimit(query.Limit),
	})
	if err != nil {
		return RuleListResult{}, err
	}

	results := make([]RuleResult, 0, len(rules))
	for _, rule := range rules {
		results = append(results, s.ruleResult(ctx, rule))
	}
	return RuleListResult{Items: results}, nil
}

func (s *service) ListEvents(ctx context.Context, query EventListQuery) (EventListResult, error) {
	cursor, err := page.Decode(query.Cursor)
	if err != nil {
		return EventListResult{}, err
	}

	limit := normalizeLimit(query.Limit)
	events, err := s.repo.FindEvents(ctx, EventQuery{
		RuleID:    query.RuleID,
		Limit:     limit + 1,
		CreatedAt: cursor.Time,
		ID:        cursor.ID,
	})
	if err != nil {
		return EventListResult{}, err
	}

	nextCursor := ""
	if len(events) > limit {
		last := events[limit-1]
		nextCursor, err = page.Encode(page.Cursor{Time: last.CreatedAt, ID: last.ID})
		if err != nil {
			return EventListResult{}, err
		}
		events = events[:limit]
	}

	results := make([]EventResult, 0, len(events))
	for _, event := range events {
		results = append(results, eventResult(event))
	}
	return EventListResult{Items: results, NextCursor: nextCursor}, nil
}

func (s *service) EnqueueForUsageEvents(ctx context.Context, events []UsageEvent) error {
	now := s.now()
	seen := map[string]struct{}{}
	for _, event := range events {
		rules, err := s.repo.FindRules(ctx, RuleQuery{
			MeterName: strings.TrimSpace(event.Meter),
			Enabled:   boolPointer(true),
			Limit:     domainusage.MaxLimit,
		})
		if err != nil {
			return err
		}

		for _, rule := range rules {
			if _, exists := seen[rule.ID]; exists {
				continue
			}
			if !ruleMatchesUsageEvent(rule, event) {
				continue
			}
			seen[rule.ID] = struct{}{}
			if err := s.repo.EnqueueEvaluationJob(ctx, rule.ID, now.Add(DefaultEvaluationDelay), now); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *service) EnqueueDueRules(ctx context.Context, limit int) (int, error) {
	return s.repo.EnqueueDueEvaluationJobs(ctx, s.now(), normalizeLimit(limit))
}

func (s *service) ClaimEvaluationJob(ctx context.Context, cmd ClaimCommand) (EvaluationJobResult, bool, error) {
	if cmd.LockTTL <= 0 || cmd.MaxAttempts <= 0 {
		return EvaluationJobResult{}, false, domain.ErrInvalidInput
	}
	now := s.now()
	job, err := s.repo.ClaimEvaluationJob(ctx, now, now.Add(cmd.LockTTL), cmd.MaxAttempts)
	if errors.Is(err, domain.ErrNotFound) {
		return EvaluationJobResult{}, false, nil
	}
	if err != nil {
		return EvaluationJobResult{}, false, err
	}
	return evaluationJobResult(job), true, nil
}

func (s *service) CompleteEvaluationJob(ctx context.Context, cmd CompleteCommand) error {
	id, err := normalizeRequired(cmd.RuleID, "alert rule id is required")
	if err != nil {
		return err
	}
	return s.repo.CompleteEvaluationJob(ctx, id)
}

func (s *service) FailEvaluationJob(ctx context.Context, cmd FailCommand) error {
	id, err := normalizeRequired(cmd.RuleID, "alert rule id is required")
	if err != nil {
		return err
	}
	if cmd.RetryAfter <= 0 {
		cmd.RetryAfter = time.Minute
	}
	now := s.now()
	return s.repo.RequeueEvaluationJob(ctx, id, now.Add(cmd.RetryAfter), now)
}

func (s *service) RecordDelivery(ctx context.Context, cmd DeliveryCommand) (DeliveryResult, error) {
	eventID, err := normalizeRequired(cmd.EventID, "alert event id is required")
	if err != nil {
		return DeliveryResult{}, err
	}
	triggerType, err := normalizeTriggerType(TriggerType(cmd.TriggerType))
	if err != nil {
		return DeliveryResult{}, err
	}
	status, err := normalizeDeliveryStatus(DeliveryStatus(cmd.Status))
	if err != nil {
		return DeliveryResult{}, err
	}
	if cmd.StatusCode < 0 {
		return DeliveryResult{}, errors.Join(domain.ErrInvalidInput, errors.New("delivery status code cannot be negative"))
	}
	if cmd.Duration < 0 {
		return DeliveryResult{}, errors.Join(domain.ErrInvalidInput, errors.New("delivery duration cannot be negative"))
	}

	now := s.now()
	attemptedAt := cmd.AttemptedAt
	if attemptedAt.IsZero() {
		attemptedAt = now
	}
	delivery := Delivery{
		ID:          uuid.NewString(),
		EventID:     eventID,
		TriggerType: triggerType,
		Status:      status,
		StatusCode:  cmd.StatusCode,
		Error:       strings.TrimSpace(cmd.Error),
		Duration:    cmd.Duration,
		AttemptedAt: attemptedAt.UTC(),
		CreatedAt:   now,
	}
	saved, err := s.repo.SaveDelivery(ctx, delivery)
	if err != nil {
		return DeliveryResult{}, err
	}
	return deliveryResult(saved), nil
}

func (s *service) Evaluate(ctx context.Context, cmd EvaluateCommand) (EvaluationResult, error) {
	rule, err := s.findRule(ctx, cmd.RuleID)
	if err != nil {
		return EvaluationResult{}, err
	}
	if !rule.Enabled {
		state := State{RuleID: rule.ID, Status: StateOK, Message: "alert rule is disabled", UpdatedAt: s.now()}
		saved, err := s.repo.SaveState(ctx, state)
		if err != nil {
			return EvaluationResult{}, err
		}
		return EvaluationResult{Rule: s.ruleResult(ctx, rule), State: stateResult(saved)}, nil
	}

	meter, err := s.findMeter(ctx, rule.MeterName)
	if err != nil {
		return EvaluationResult{}, err
	}

	if rule.GroupBy != "" {
		return s.evaluateGrouped(ctx, rule, meter)
	}
	return s.evaluateGlobal(ctx, rule, meter)
}

func (s *service) evaluateGlobal(ctx context.Context, rule Rule, meter domainmeter.Meter) (EvaluationResult, error) {
	now := s.now()
	query, err := domainusage.NewAggregateQuery(
		rule.Subject,
		rule.MeterName,
		now.Add(-rule.Window),
		now,
		meter.Aggregation(),
		rule.Metadata,
		domainusage.EmptyFilter(),
	)
	if err != nil {
		return EvaluationResult{}, err
	}
	aggregate, err := s.usageRepo.Aggregate(ctx, query)
	if err != nil {
		state := State{
			RuleID:      rule.ID,
			Status:      StateError,
			Message:     err.Error(),
			EvaluatedAt: now,
			UpdatedAt:   now,
		}
		event := Event{ID: uuid.NewString(), RuleID: rule.ID, Type: EventEvaluationFailed, Message: err.Error(), CreatedAt: now}
		if saveErr := s.saveEvaluation(ctx, rule, state, &event, now); saveErr != nil {
			return EvaluationResult{}, errors.Join(err, saveErr)
		}
		return EvaluationResult{}, err
	}

	nextState := State{
		RuleID:      rule.ID,
		Value:       aggregate.Quantity(),
		EvaluatedAt: now,
		UpdatedAt:   now,
	}
	if aggregate.UsageEvents() == 0 {
		nextState.Status = StateNoData
		nextState.Message = "no usage data in alert window"
	} else if compare(rule.Comparator, aggregate.Quantity(), rule.Threshold) {
		nextState.Status = StateAlerting
		nextState.Message = fmt.Sprintf("value %.4f %s threshold %.4f", aggregate.Quantity(), rule.Comparator, rule.Threshold)
	} else {
		nextState.Status = StateOK
		nextState.Message = fmt.Sprintf("value %.4f is within threshold %.4f", aggregate.Quantity(), rule.Threshold)
	}

	previous, found, err := s.repo.FindState(ctx, rule.ID, "", "")
	if err != nil {
		return EvaluationResult{}, err
	}

	var event *Event
	switch {
	case nextState.Status == StateAlerting && (!found || previous.Status != StateAlerting):
		event = &Event{ID: uuid.NewString(), RuleID: rule.ID, Type: EventTriggered, Value: nextState.Value, Message: nextState.Message, CreatedAt: now}
	case found && previous.Status == StateAlerting && nextState.Status != StateAlerting:
		event = &Event{ID: uuid.NewString(), RuleID: rule.ID, Type: EventResolved, Value: nextState.Value, Message: nextState.Message, CreatedAt: now}
	}

	if err := s.saveEvaluation(ctx, rule, nextState, event, now); err != nil {
		return EvaluationResult{}, err
	}

	result := EvaluationResult{Rule: s.ruleResult(ctx, rule), State: stateResult(nextState)}
	if event != nil {
		eventResult := eventResult(*event)
		result.Event = &eventResult
		result.Events = []EventResult{eventResult}
	}
	return result, nil
}

func (s *service) evaluateGrouped(ctx context.Context, rule Rule, meter domainmeter.Meter) (EvaluationResult, error) {
	now := s.now()
	filter, err := metadataFilter(rule.Metadata)
	if err != nil {
		return EvaluationResult{}, err
	}
	query, err := domainusage.NewBreakdownQuery(
		rule.MeterName,
		rule.GroupBy,
		rule.Subject,
		now.Add(-rule.Window),
		now,
		meter.Aggregation(),
		domainusage.MaxLimit,
		filter,
	)
	if err != nil {
		return EvaluationResult{}, err
	}
	items, err := s.usageRepo.FindBreakdown(ctx, query)
	if err != nil {
		state := State{
			RuleID:      rule.ID,
			Status:      StateError,
			Message:     err.Error(),
			EvaluatedAt: now,
			UpdatedAt:   now,
		}
		event := Event{ID: uuid.NewString(), RuleID: rule.ID, Type: EventEvaluationFailed, Message: err.Error(), CreatedAt: now}
		if saveErr := s.saveEvaluation(ctx, rule, state, &event, now); saveErr != nil {
			return EvaluationResult{}, errors.Join(err, saveErr)
		}
		return EvaluationResult{}, err
	}

	previousStates, err := s.repo.FindStates(ctx, rule.ID, domainusage.MaxLimit)
	if err != nil {
		return EvaluationResult{}, err
	}
	previousByGroup := map[string]State{}
	for _, state := range previousStates {
		previousByGroup[groupID(state.GroupKey, state.GroupValue)] = state
	}

	states := make([]State, 0, len(items)+len(previousStates))
	events := []Event{}
	seen := map[string]struct{}{}
	for _, item := range items {
		state := groupedState(rule, rule.GroupBy, item.Value(), item.Quantity(), item.UsageEvents(), now)
		key := groupID(state.GroupKey, state.GroupValue)
		seen[key] = struct{}{}
		if previous, found := previousByGroup[key]; shouldCreateEvent(previous, found, state) {
			events = append(events, eventForState(rule.ID, state, found, now))
		}
		states = append(states, state)
	}

	for _, previous := range previousStates {
		key := groupID(previous.GroupKey, previous.GroupValue)
		if _, exists := seen[key]; exists || previous.GroupKey == "" {
			continue
		}
		state := State{
			RuleID:      rule.ID,
			GroupKey:    previous.GroupKey,
			GroupValue:  previous.GroupValue,
			Status:      StateNoData,
			Message:     groupMessage(previous.GroupKey, previous.GroupValue, "no usage data in alert window"),
			EvaluatedAt: now,
			UpdatedAt:   now,
		}
		if previous.Status == StateAlerting {
			events = append(events, Event{
				ID:         uuid.NewString(),
				RuleID:     rule.ID,
				GroupKey:   state.GroupKey,
				GroupValue: state.GroupValue,
				Type:       EventResolved,
				Value:      state.Value,
				Message:    state.Message,
				CreatedAt:  now,
			})
		}
		states = append(states, state)
	}

	if len(states) == 0 {
		state := State{
			RuleID:      rule.ID,
			Status:      StateNoData,
			Message:     "no usage data in alert window",
			EvaluatedAt: now,
			UpdatedAt:   now,
		}
		states = append(states, state)
	}

	if err := s.saveEvaluations(ctx, rule, states, events, now); err != nil {
		return EvaluationResult{}, err
	}

	state := primaryState(states)
	result := EvaluationResult{Rule: s.ruleResult(ctx, rule), State: stateResult(state)}
	for _, event := range events {
		eventResult := eventResult(event)
		result.Events = append(result.Events, eventResult)
	}
	if len(result.Events) > 0 {
		result.Event = &result.Events[0]
	}
	return result, nil
}

func (s *service) saveEvaluation(ctx context.Context, rule Rule, state State, event *Event, now time.Time) error {
	events := []Event{}
	if event != nil {
		events = append(events, *event)
	}
	return s.saveEvaluations(ctx, rule, []State{state}, events, now)
}

func (s *service) saveEvaluations(ctx context.Context, rule Rule, states []State, events []Event, now time.Time) error {
	return s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		for _, state := range states {
			if _, err := s.repo.SaveState(txCtx, state); err != nil {
				return err
			}
		}
		for _, event := range events {
			if _, err := s.repo.SaveEvent(txCtx, event); err != nil {
				return err
			}
		}
		return s.repo.UpdateRuleNextEvaluation(txCtx, rule.ID, now.Add(rule.EvaluationInterval), now)
	})
}

func (s *service) findRule(ctx context.Context, id string) (Rule, error) {
	id, err := normalizeRequired(id, "alert id is required")
	if err != nil {
		return Rule{}, err
	}
	rules, err := s.repo.FindRules(ctx, RuleQuery{ID: id, Limit: 1})
	if err != nil {
		return Rule{}, err
	}
	if len(rules) == 0 {
		return Rule{}, domain.ErrNotFound
	}
	return rules[0], nil
}

func (s *service) findDestination(ctx context.Context, id string) (Destination, error) {
	id, err := normalizeRequired(id, "alert destination id is required")
	if err != nil {
		return Destination{}, err
	}
	destinations, err := s.repo.FindDestinations(ctx, DestinationQuery{ID: id, Limit: 1})
	if err != nil {
		return Destination{}, err
	}
	if len(destinations) == 0 {
		return Destination{}, domain.ErrNotFound
	}
	return destinations[0], nil
}

func (s *service) validateRequiredDestinationReference(ctx context.Context, id string) error {
	id, err := normalizeRequired(id, "alert destination is required")
	if err != nil {
		return err
	}
	_, err = s.findDestination(ctx, id)
	return err
}

func (s *service) findMeter(ctx context.Context, meterName string) (domainmeter.Meter, error) {
	meters, err := s.meterRepo.Find(ctx, domainmeter.Query{Name: strings.TrimSpace(meterName)})
	if err != nil {
		return domainmeter.Meter{}, err
	}
	if len(meters) == 0 {
		return domainmeter.Meter{}, domain.ErrNotFound
	}
	return meters[0], nil
}

func (s *service) ruleResult(ctx context.Context, rule Rule) RuleResult {
	result := ruleResult(rule)
	if rule.DestinationID != "" {
		if destination, err := s.findDestination(ctx, rule.DestinationID); err == nil {
			value := destinationResult(destination)
			result.Destination = &value
		}
	}
	states, err := s.repo.FindStates(ctx, rule.ID, domainusage.MaxLimit)
	if err == nil && len(states) > 0 {
		results := make([]StateResult, 0, len(states))
		for _, state := range states {
			results = append(results, stateResult(state))
		}
		result.States = results
		primary := stateResult(primaryState(states))
		result.State = &primary
	} else {
		state, found, err := s.repo.FindState(ctx, rule.ID, "", "")
		if err == nil && found {
			value := stateResult(state)
			result.State = &value
		}
	}
	return result
}

func ruleFromInput(existing Rule, cmd SaveCommand, now time.Time) (Rule, error) {
	enabled := true
	if cmd.Enabled != nil {
		enabled = *cmd.Enabled
	}
	rule := Rule{
		ID:                 existing.ID,
		Name:               cmd.Name,
		MeterName:          cmd.MeterName,
		Enabled:            enabled,
		Subject:            cmd.Subject,
		Metadata:           cmd.Metadata,
		Window:             cmd.Window,
		Comparator:         Comparator(cmd.Comparator),
		Threshold:          cmd.Threshold,
		EvaluationInterval: cmd.EvaluationInterval,
		GroupBy:            cmd.GroupBy,
		DestinationID:      cmd.DestinationID,
		NextEvaluateAt:     existing.NextEvaluateAt,
		CreatedAt:          existing.CreatedAt,
		UpdatedAt:          now,
	}
	return validateRule(rule, now)
}

func destinationFromInput(existing Destination, cmd DestinationSaveCommand, now time.Time) (Destination, error) {
	enabled := true
	if cmd.Enabled != nil {
		enabled = *cmd.Enabled
	}
	destination := Destination{
		ID:            existing.ID,
		Name:          cmd.Name,
		Type:          DestinationType(cmd.Type),
		Enabled:       enabled,
		WebhookURL:    cmd.WebhookURL,
		WebhookSecret: existing.WebhookSecret,
		CreatedAt:     existing.CreatedAt,
		UpdatedAt:     now,
	}
	return validateDestination(destination, now)
}

func validateDestination(destination Destination, now time.Time) (Destination, error) {
	name, err := normalizeName(destination.Name)
	if err != nil {
		return Destination{}, err
	}
	destinationType, err := normalizeDestinationType(destination.Type)
	if err != nil {
		return Destination{}, err
	}
	webhookURL, err := normalizeWebhookURL(destination.WebhookURL)
	if err != nil {
		return Destination{}, err
	}
	if destinationType == DestinationWebhook && webhookURL == "" {
		return Destination{}, errors.Join(domain.ErrInvalidInput, errors.New("webhook url is required"))
	}
	if destinationType == DestinationWebhook && strings.TrimSpace(destination.WebhookSecret) == "" {
		secret, err := newWebhookSecret()
		if err != nil {
			return Destination{}, err
		}
		destination.WebhookSecret = secret
	}
	if destination.CreatedAt.IsZero() {
		destination.CreatedAt = now
	}
	if destination.UpdatedAt.IsZero() {
		destination.UpdatedAt = now
	}

	destination.Name = name
	destination.Type = destinationType
	destination.WebhookURL = webhookURL
	destination.WebhookSecret = strings.TrimSpace(destination.WebhookSecret)
	destination.CreatedAt = destination.CreatedAt.UTC()
	destination.UpdatedAt = destination.UpdatedAt.UTC()
	return destination, nil
}

func validateRule(rule Rule, now time.Time) (Rule, error) {
	name, err := normalizeName(rule.Name)
	if err != nil {
		return Rule{}, err
	}
	meterName, err := normalizeRequired(rule.MeterName, "meter is required")
	if err != nil {
		return Rule{}, err
	}
	subject := strings.TrimSpace(rule.Subject)
	metadata, err := normalizeMetadata(rule.Metadata)
	if err != nil {
		return Rule{}, err
	}
	window, err := normalizeDuration(rule.Window, time.Minute, 366*24*time.Hour, "window")
	if err != nil {
		return Rule{}, err
	}
	if window == 0 {
		window = DefaultWindow
	}
	comparator, err := normalizeComparator(rule.Comparator)
	if err != nil {
		return Rule{}, err
	}
	if math.IsNaN(rule.Threshold) || math.IsInf(rule.Threshold, 0) {
		return Rule{}, errors.Join(domain.ErrInvalidInput, errors.New("threshold must be finite"))
	}
	interval, err := normalizeDuration(rule.EvaluationInterval, time.Second, 24*time.Hour, "evaluation interval")
	if err != nil {
		return Rule{}, err
	}
	if interval == 0 {
		interval = DefaultEvaluationInterval
	}
	groupBy, err := normalizeGroupBy(rule.GroupBy)
	if err != nil {
		return Rule{}, err
	}
	destinationID, err := normalizeRequired(rule.DestinationID, "alert destination is required")
	if err != nil {
		return Rule{}, err
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = now
	}
	if rule.UpdatedAt.IsZero() {
		rule.UpdatedAt = now
	}
	if rule.NextEvaluateAt.IsZero() {
		rule.NextEvaluateAt = now
	}
	rule.Name = name
	rule.MeterName = meterName
	rule.Subject = subject
	rule.Metadata = metadata
	rule.Window = window
	rule.Comparator = comparator
	rule.EvaluationInterval = interval
	rule.GroupBy = groupBy
	rule.DestinationID = destinationID
	rule.CreatedAt = rule.CreatedAt.UTC()
	rule.UpdatedAt = rule.UpdatedAt.UTC()
	rule.NextEvaluateAt = rule.NextEvaluateAt.UTC()
	return rule, nil
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

func normalizeRequired(value string, message string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.Join(domain.ErrInvalidInput, errors.New(message))
	}
	return value, nil
}

func normalizeDuration(value time.Duration, min time.Duration, max time.Duration, label string) (time.Duration, error) {
	if value == 0 {
		return 0, nil
	}
	if value < min {
		return 0, errors.Join(domain.ErrInvalidInput, fmt.Errorf("%s must be at least %s", label, min))
	}
	if value > max {
		return 0, errors.Join(domain.ErrInvalidInput, fmt.Errorf("%s cannot exceed %s", label, max))
	}
	return value, nil
}

func normalizeComparator(value Comparator) (Comparator, error) {
	switch value {
	case "", ComparatorGreaterThanOrEqual:
		return ComparatorGreaterThanOrEqual, nil
	case ComparatorGreaterThan, ComparatorLessThan, ComparatorLessThanOrEqual, ComparatorEqual, ComparatorNotEqual:
		return value, nil
	default:
		return "", errors.Join(domain.ErrInvalidInput, fmt.Errorf("unsupported comparator %q", value))
	}
}

func normalizeGroupBy(value string) (string, error) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "metadata.")
	if value == "" {
		return "", nil
	}
	if domainusage.IsSubjectGroupBy(value) {
		return value, nil
	}
	if _, err := domainmeter.NewDimension(value, domainmeter.MetadataString, "", "", false); err != nil {
		return "", errors.Join(domain.ErrInvalidInput, fmt.Errorf("unsupported alert group field %q", value))
	}
	return value, nil
}

func normalizeTriggerType(value TriggerType) (TriggerType, error) {
	switch value {
	case "", TriggerWebhook:
		return TriggerWebhook, nil
	default:
		return "", errors.Join(domain.ErrInvalidInput, fmt.Errorf("unsupported trigger type %q", value))
	}
}

func normalizeDestinationType(value DestinationType) (DestinationType, error) {
	switch value {
	case "", DestinationWebhook:
		return DestinationWebhook, nil
	default:
		return "", errors.Join(domain.ErrInvalidInput, fmt.Errorf("unsupported destination type %q", value))
	}
}

func normalizeDeliveryStatus(value DeliveryStatus) (DeliveryStatus, error) {
	switch value {
	case DeliveryDelivered, DeliveryFailed:
		return value, nil
	default:
		return "", errors.Join(domain.ErrInvalidInput, fmt.Errorf("unsupported delivery status %q", value))
	}
}

func normalizeWebhookURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.Join(domain.ErrInvalidInput, errors.New("webhook url must be an absolute http or https url"))
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.Join(domain.ErrInvalidInput, errors.New("webhook url must use http or https"))
	}
	return value, nil
}

func normalizeMetadata(input map[string]string) (map[string]string, error) {
	metadata := map[string]string{}
	for key, value := range input {
		key = strings.TrimPrefix(strings.TrimSpace(key), "metadata.")
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, errors.Join(domain.ErrInvalidInput, errors.New("metadata filter key is required"))
		}
		if value == "" {
			return nil, errors.Join(domain.ErrInvalidInput, errors.New("metadata filter value is required"))
		}
		metadata[key] = value
	}
	return metadata, nil
}

func normalizeLimit(value int) int {
	if value <= 0 {
		return DefaultJobLimit
	}
	if value > domainusage.MaxLimit {
		return domainusage.MaxLimit
	}
	return value
}

func compare(comparator Comparator, value float64, threshold float64) bool {
	switch comparator {
	case ComparatorGreaterThan:
		return value > threshold
	case ComparatorGreaterThanOrEqual:
		return value >= threshold
	case ComparatorLessThan:
		return value < threshold
	case ComparatorLessThanOrEqual:
		return value <= threshold
	case ComparatorEqual:
		return value == threshold
	case ComparatorNotEqual:
		return value != threshold
	default:
		return value >= threshold
	}
}

func ruleMatchesUsageEvent(rule Rule, event UsageEvent) bool {
	if rule.Subject != "" && rule.Subject != event.Subject {
		return false
	}
	for key, expected := range rule.Metadata {
		actual, ok := metadataValue(event.Metadata, key)
		if !ok || fmt.Sprint(actual) != expected {
			return false
		}
	}
	return true
}

func metadataFilter(metadata map[string]string) (domainusage.Filter, error) {
	conditions := make([]domainusage.Filter, 0, len(metadata))
	for key, value := range metadata {
		condition, err := domainusage.NewFilterCondition("metadata."+key, domainusage.FilterOpEqual, value, true)
		if err != nil {
			return domainusage.Filter{}, err
		}
		conditions = append(conditions, condition)
	}
	if len(conditions) == 0 {
		return domainusage.EmptyFilter(), nil
	}
	if len(conditions) == 1 {
		return conditions[0], nil
	}
	return domainusage.NewFilterGroup(domainusage.FilterGroupAnd, conditions)
}

func groupedState(rule Rule, groupKey string, groupValue string, value float64, usageEvents int, now time.Time) State {
	state := State{
		RuleID:      rule.ID,
		GroupKey:    groupKey,
		GroupValue:  groupValue,
		Value:       value,
		EvaluatedAt: now,
		UpdatedAt:   now,
	}
	if usageEvents == 0 {
		state.Status = StateNoData
		state.Message = groupMessage(groupKey, groupValue, "no usage data in alert window")
	} else if compare(rule.Comparator, value, rule.Threshold) {
		state.Status = StateAlerting
		state.Message = groupMessage(groupKey, groupValue, fmt.Sprintf("value %.4f %s threshold %.4f", value, rule.Comparator, rule.Threshold))
	} else {
		state.Status = StateOK
		state.Message = groupMessage(groupKey, groupValue, fmt.Sprintf("value %.4f is within threshold %.4f", value, rule.Threshold))
	}
	return state
}

func shouldCreateEvent(previous State, found bool, next State) bool {
	return next.Status == StateAlerting && (!found || previous.Status != StateAlerting) ||
		found && previous.Status == StateAlerting && next.Status != StateAlerting
}

func eventForState(ruleID string, state State, found bool, now time.Time) Event {
	eventType := EventTriggered
	if found && state.Status != StateAlerting {
		eventType = EventResolved
	}
	return Event{
		ID:         uuid.NewString(),
		RuleID:     ruleID,
		GroupKey:   state.GroupKey,
		GroupValue: state.GroupValue,
		Type:       eventType,
		Value:      state.Value,
		Message:    state.Message,
		CreatedAt:  now,
	}
}

func groupID(groupKey string, groupValue string) string {
	return groupKey + "\x00" + groupValue
}

func groupMessage(groupKey string, groupValue string, message string) string {
	if groupKey == "" && groupValue == "" {
		return message
	}
	return fmt.Sprintf("%s for %s %s", message, groupKey, groupValue)
}

func primaryState(states []State) State {
	if len(states) == 0 {
		return State{}
	}
	primary := states[0]
	for _, state := range states[1:] {
		if statePriority(state.Status) < statePriority(primary.Status) {
			primary = state
			continue
		}
		if statePriority(state.Status) == statePriority(primary.Status) && state.UpdatedAt.After(primary.UpdatedAt) {
			primary = state
		}
	}
	return primary
}

func statePriority(status StateStatus) int {
	switch status {
	case StateAlerting:
		return 0
	case StateError:
		return 1
	case StateNoData:
		return 2
	default:
		return 3
	}
}

func metadataValue(metadata map[string]any, key string) (any, bool) {
	if value, ok := metadata[key]; ok {
		return value, true
	}
	parts := strings.Split(key, ".")
	if len(parts) == 1 {
		return nil, false
	}
	var current any = metadata
	for _, part := range parts {
		node, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		value, exists := node[part]
		if !exists {
			return nil, false
		}
		current = value
	}
	return current, true
}

func boolPointer(value bool) *bool {
	return &value
}

func newWebhookSecret() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return WebhookSecretPrefix + base64.RawURLEncoding.EncodeToString(random), nil
}

func ruleResult(rule Rule) RuleResult {
	return RuleResult{
		ID:                 rule.ID,
		Name:               rule.Name,
		MeterName:          rule.MeterName,
		Enabled:            rule.Enabled,
		Subject:            rule.Subject,
		Metadata:           cloneMetadata(rule.Metadata),
		WindowSeconds:      int(rule.Window.Seconds()),
		Comparator:         string(rule.Comparator),
		Threshold:          rule.Threshold,
		EvaluationInterval: int(rule.EvaluationInterval.Seconds()),
		GroupBy:            rule.GroupBy,
		DestinationID:      rule.DestinationID,
		NextEvaluateAt:     rule.NextEvaluateAt,
		CreatedAt:          rule.CreatedAt,
		UpdatedAt:          rule.UpdatedAt,
	}
}

func destinationResult(destination Destination) DestinationResult {
	return DestinationResult{
		ID:              destination.ID,
		Name:            destination.Name,
		Type:            string(destination.Type),
		Enabled:         destination.Enabled,
		WebhookURL:      destination.WebhookURL,
		WebhookSecret:   destination.WebhookSecret,
		SignatureHeader: WebhookSignatureHeader,
		TimestampHeader: WebhookTimestampHeader,
		Algorithm:       WebhookSignatureAlgorithm,
		CreatedAt:       destination.CreatedAt,
		UpdatedAt:       destination.UpdatedAt,
	}
}

func stateResult(state State) StateResult {
	return StateResult{
		Status:      string(state.Status),
		GroupKey:    state.GroupKey,
		GroupValue:  state.GroupValue,
		Value:       state.Value,
		Message:     state.Message,
		EvaluatedAt: state.EvaluatedAt,
		UpdatedAt:   state.UpdatedAt,
	}
}

func eventResult(event Event) EventResult {
	result := EventResult{
		ID:         event.ID,
		RuleID:     event.RuleID,
		GroupKey:   event.GroupKey,
		GroupValue: event.GroupValue,
		Type:       string(event.Type),
		Value:      event.Value,
		Message:    event.Message,
		CreatedAt:  event.CreatedAt,
	}
	if event.Delivery != nil {
		delivery := deliveryResult(*event.Delivery)
		result.Delivery = &delivery
	}
	return result
}

func deliveryResult(delivery Delivery) DeliveryResult {
	return DeliveryResult{
		ID:          delivery.ID,
		EventID:     delivery.EventID,
		TriggerType: string(delivery.TriggerType),
		Status:      string(delivery.Status),
		StatusCode:  delivery.StatusCode,
		Error:       delivery.Error,
		DurationMs:  int(delivery.Duration.Milliseconds()),
		AttemptedAt: delivery.AttemptedAt,
		CreatedAt:   delivery.CreatedAt,
	}
}

func evaluationJobResult(job EvaluationJob) EvaluationJobResult {
	return EvaluationJobResult{
		WorkspaceID: job.WorkspaceID,
		RuleID:      job.RuleID,
		RunAfter:    job.RunAfter,
		LockedUntil: job.LockedUntil,
		Attempts:    job.Attempts,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
	}
}

func cloneMetadata(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
