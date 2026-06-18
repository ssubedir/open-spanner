package alert

import (
	"context"
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

type Repository interface {
	SaveRule(ctx context.Context, rule Rule) (Rule, error)
	FindRules(ctx context.Context, query RuleQuery) ([]Rule, error)
	DeleteRule(ctx context.Context, id string) error
	SaveState(ctx context.Context, state State) (State, error)
	FindState(ctx context.Context, ruleID string) (State, bool, error)
	SaveEvent(ctx context.Context, event Event) (Event, error)
	FindEvents(ctx context.Context, query EventQuery) ([]Event, error)
	EnqueueEvaluationJob(ctx context.Context, ruleID string, runAfter time.Time, now time.Time) error
	EnqueueDueEvaluationJobs(ctx context.Context, now time.Time, limit int) (int, error)
	ClaimEvaluationJob(ctx context.Context, now time.Time, lockedUntil time.Time, maxAttempts int) (EvaluationJob, error)
	CompleteEvaluationJob(ctx context.Context, ruleID string) error
	RequeueEvaluationJob(ctx context.Context, ruleID string, runAfter time.Time, now time.Time) error
	UpdateRuleNextEvaluation(ctx context.Context, id string, nextEvaluateAt time.Time, updatedAt time.Time) error
}

type UsageRepository interface {
	Aggregate(ctx context.Context, query domainusage.AggregateQuery) (domainusage.Aggregate, error)
}

type Service interface {
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
	TriggerType        TriggerType
	WebhookURL         string
	NextEvaluateAt     time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type State struct {
	RuleID      string
	Status      StateStatus
	Value       float64
	Message     string
	EvaluatedAt time.Time
	UpdatedAt   time.Time
}

type Event struct {
	ID        string
	RuleID    string
	Type      EventType
	Value     float64
	Message   string
	CreatedAt time.Time
}

type EvaluationJob struct {
	RuleID      string
	RunAfter    time.Time
	LockedUntil time.Time
	Attempts    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type RuleQuery struct {
	ID        string
	MeterName string
	Enabled   *bool
	Limit     int
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
	TriggerType        string
	WebhookURL         string
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
	TriggerType        string
	WebhookURL         *string
}

type DeleteCommand struct {
	ID string
}

type GetQuery struct {
	ID string
}

type ListQuery struct {
	MeterName string
	Enabled   *bool
	Limit     int
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
	TriggerType        string
	WebhookURL         string
	NextEvaluateAt     time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	State              *StateResult
}

type StateResult struct {
	Status      string
	Value       float64
	Message     string
	EvaluatedAt time.Time
	UpdatedAt   time.Time
}

type RuleListResult struct {
	Items []RuleResult
}

type EventResult struct {
	ID        string
	RuleID    string
	Type      string
	Value     float64
	Message   string
	CreatedAt time.Time
}

type EventListResult struct {
	Items      []EventResult
	NextCursor string
}

type EvaluationJobResult struct {
	RuleID      string
	RunAfter    time.Time
	LockedUntil time.Time
	Attempts    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type EvaluationResult struct {
	Rule  RuleResult
	State StateResult
	Event *EventResult
}

func (s *service) Create(ctx context.Context, cmd SaveCommand) (RuleResult, error) {
	if _, err := s.findMeter(ctx, cmd.MeterName); err != nil {
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
		TriggerType:        cmd.TriggerType,
		WebhookURL:         cmd.WebhookURL,
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
	if cmd.TriggerType != "" {
		next.TriggerType = TriggerType(cmd.TriggerType)
	}
	if cmd.WebhookURL != nil {
		next.WebhookURL = *cmd.WebhookURL
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
		MeterName: query.MeterName,
		Enabled:   query.Enabled,
		Limit:     normalizeLimit(query.Limit),
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

	previous, found, err := s.repo.FindState(ctx, rule.ID)
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
	}
	return result, nil
}

func (s *service) saveEvaluation(ctx context.Context, rule Rule, state State, event *Event, now time.Time) error {
	return s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		if _, err := s.repo.SaveState(txCtx, state); err != nil {
			return err
		}
		if event != nil {
			if _, err := s.repo.SaveEvent(txCtx, *event); err != nil {
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
	state, found, err := s.repo.FindState(ctx, rule.ID)
	if err == nil && found {
		value := stateResult(state)
		result.State = &value
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
		TriggerType:        TriggerType(cmd.TriggerType),
		WebhookURL:         cmd.WebhookURL,
		NextEvaluateAt:     existing.NextEvaluateAt,
		CreatedAt:          existing.CreatedAt,
		UpdatedAt:          now,
	}
	return validateRule(rule, now)
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
	triggerType, err := normalizeTriggerType(rule.TriggerType)
	if err != nil {
		return Rule{}, err
	}
	webhookURL, err := normalizeWebhookURL(rule.WebhookURL)
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
	rule.TriggerType = triggerType
	rule.WebhookURL = webhookURL
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

func normalizeTriggerType(value TriggerType) (TriggerType, error) {
	switch value {
	case "", TriggerWebhook:
		return TriggerWebhook, nil
	default:
		return "", errors.Join(domain.ErrInvalidInput, fmt.Errorf("unsupported trigger type %q", value))
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
		TriggerType:        string(rule.TriggerType),
		WebhookURL:         rule.WebhookURL,
		NextEvaluateAt:     rule.NextEvaluateAt,
		CreatedAt:          rule.CreatedAt,
		UpdatedAt:          rule.UpdatedAt,
	}
}

func stateResult(state State) StateResult {
	return StateResult{
		Status:      string(state.Status),
		Value:       state.Value,
		Message:     state.Message,
		EvaluatedAt: state.EvaluatedAt,
		UpdatedAt:   state.UpdatedAt,
	}
}

func eventResult(event Event) EventResult {
	return EventResult{
		ID:        event.ID,
		RuleID:    event.RuleID,
		Type:      string(event.Type),
		Value:     event.Value,
		Message:   event.Message,
		CreatedAt: event.CreatedAt,
	}
}

func evaluationJobResult(job EvaluationJob) EvaluationJobResult {
	return EvaluationJobResult{
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
