package entitlement

import (
	"context"
	"errors"
	"fmt"
	"math"
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
	MaxNameRunes               = 120
	DefaultLimit               = 100
	MaxLimit                   = 1000
	DefaultWarningPercent      = 80
	DefaultCheckEvaluationWait = 0
)

type Period string

const (
	PeriodDay   Period = "day"
	PeriodWeek  Period = "week"
	PeriodMonth Period = "month"
	PeriodYear  Period = "year"
)

type OverageState string

const (
	StateOK        OverageState = "ok"
	StateWarning   OverageState = "warning"
	StateExceeded  OverageState = "exceeded"
	StateNoPlan    OverageState = "no_plan"
	StateNotInPlan OverageState = "not_in_plan"
)

type EventType string

const (
	EventWarning   EventType = "warning"
	EventExceeded  EventType = "exceeded"
	EventRecovered EventType = "recovered"
)

type Repository interface {
	SavePlan(ctx context.Context, plan Plan) (Plan, error)
	FindPlans(ctx context.Context, query PlanQuery) ([]Plan, error)
	DeletePlan(ctx context.Context, id string) error
	ReplacePlanLimits(ctx context.Context, planID string, limits []PlanLimit) error
	FindPlanLimits(ctx context.Context, planID string) ([]PlanLimit, error)
	CountAssignmentsForPlan(ctx context.Context, planID string) (int, error)
	SaveSubjectAssignment(ctx context.Context, assignment SubjectAssignment) (SubjectAssignment, error)
	FindSubjectAssignments(ctx context.Context, query AssignmentQuery) ([]SubjectAssignment, error)
	DeleteSubjectAssignment(ctx context.Context, subject string) error
	GetEntitlementState(ctx context.Context, query StateQuery) (EntitlementState, error)
	FindEntitlementStates(ctx context.Context, query StateListQuery) ([]EntitlementState, error)
	FindEntitlementEvents(ctx context.Context, query EventQuery) ([]EntitlementEvent, error)
	GetEntitlementUsageCounter(ctx context.Context, query CounterQuery) (EntitlementUsageCounter, error)
	SaveEntitlementState(ctx context.Context, state EntitlementState) error
	SaveEntitlementEvent(ctx context.Context, event EntitlementEvent) error
	EnqueueEntitlementCheckJob(ctx context.Context, job CheckJob) error
	ClaimEntitlementCheckJob(ctx context.Context, cmd ClaimCommand) (CheckJob, bool, error)
	RequeueEntitlementCheckJob(ctx context.Context, cmd FailCommand) error
	DeleteEntitlementCheckJob(ctx context.Context, cmd CompleteCommand) error
}

type UsageRepository interface {
	Aggregate(ctx context.Context, query domainusage.AggregateQuery) (domainusage.Aggregate, error)
}

type Service interface {
	CreatePlan(ctx context.Context, cmd SavePlanCommand) (PlanResult, error)
	UpdatePlan(ctx context.Context, cmd UpdatePlanCommand) (PlanResult, error)
	DeletePlan(ctx context.Context, cmd DeletePlanCommand) error
	GetPlan(ctx context.Context, query GetPlanQuery) (PlanResult, error)
	ListPlans(ctx context.Context, query ListPlansQuery) (PlanListResult, error)
	AssignSubject(ctx context.Context, cmd AssignSubjectCommand) (SubjectAssignmentResult, error)
	DeleteSubjectAssignment(ctx context.Context, cmd DeleteSubjectAssignmentCommand) error
	ListSubjectAssignments(ctx context.Context, query AssignmentListQuery) (SubjectAssignmentListResult, error)
	GetSubjectProgress(ctx context.Context, query SubjectProgressQuery) (SubjectProgressResult, error)
	Check(ctx context.Context, cmd CheckCommand) (EntitlementCheckResult, error)
	ListEntitlementStates(ctx context.Context, query StateListQuery) (StateListResult, error)
	ListEntitlementEvents(ctx context.Context, query EventListQuery) (EventListResult, error)
	EnqueueForUsageEvents(ctx context.Context, events []UsageEvent) error
	ClaimCheckJob(ctx context.Context, cmd ClaimCommand) (CheckJobResult, bool, error)
	Evaluate(ctx context.Context, cmd EvaluateCommand) (EvaluationResult, error)
	CompleteCheckJob(ctx context.Context, cmd CompleteCommand) error
	FailCheckJob(ctx context.Context, cmd FailCommand) error
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
		panic("entitlement service requires a transactor")
	}

	return &service{
		repo:       repo,
		meterRepo:  meterRepo,
		usageRepo:  usageRepo,
		transactor: transactor,
		now:        func() time.Time { return time.Now().UTC() },
	}
}

type Plan struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PlanLimit struct {
	ID             string
	PlanID         string
	MeterName      string
	Period         Period
	Limit          float64
	WarningPercent float64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type SubjectAssignment struct {
	Subject    string
	PlanID     string
	PlanName   string
	AssignedAt time.Time
	UpdatedAt  time.Time
}

type EntitlementState struct {
	WorkspaceID    string
	Subject        string
	MeterName      string
	PlanID         string
	PlanName       string
	Period         Period
	State          OverageState
	Current        float64
	Limit          float64
	Remaining      float64
	WarningPercent float64
	Message        string
	EvaluatedAt    time.Time
	UpdatedAt      time.Time
}

type EntitlementEvent struct {
	ID             string
	WorkspaceID    string
	Subject        string
	MeterName      string
	PlanID         string
	PlanName       string
	Period         Period
	PreviousState  OverageState
	State          OverageState
	Type           EventType
	Current        float64
	Limit          float64
	Remaining      float64
	WarningPercent float64
	Message        string
	CreatedAt      time.Time
}

type EntitlementUsageCounter struct {
	WorkspaceID    string
	Subject        string
	MeterName      string
	Period         Period
	From           time.Time
	To             time.Time
	EventCount     int64
	QuantitySum    float64
	QuantityMin    float64
	QuantityMax    float64
	FirstQuantity  float64
	FirstEventTime time.Time
	LastQuantity   float64
	LastEventTime  time.Time
	UpdatedAt      time.Time
}

type CheckJob struct {
	WorkspaceID string
	Subject     string
	MeterName   string
	RunAfter    time.Time
	LockedUntil time.Time
	Attempts    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PlanQuery struct {
	ID    string
	Name  string
	Limit int
}

type AssignmentQuery struct {
	Subject string
	PlanID  string
	Limit   int
}

type StateQuery struct {
	Subject   string
	MeterName string
	PlanID    string
	Period    Period
}

type StateListQuery struct {
	Subject   string
	MeterName string
	State     OverageState
	Limit     int
}

type EventQuery struct {
	Subject   string
	MeterName string
	PlanID    string
	State     OverageState
	Type      EventType
	CreatedAt time.Time
	ID        string
	Limit     int
}

type CounterQuery struct {
	Subject   string
	MeterName string
	Period    Period
	From      time.Time
}

type EventListQuery struct {
	Subject   string
	MeterName string
	PlanID    string
	State     OverageState
	Type      EventType
	Cursor    string
	Limit     int
}

type LimitCommand struct {
	MeterName      string
	Period         string
	Limit          float64
	WarningPercent float64
}

type SavePlanCommand struct {
	Name        string
	Description string
	Limits      []LimitCommand
}

type UpdatePlanCommand struct {
	ID          string
	Name        string
	Description string
	Limits      []LimitCommand
}

type DeletePlanCommand struct {
	ID string
}

type GetPlanQuery struct {
	ID string
}

type ListPlansQuery struct {
	Limit int
}

type AssignSubjectCommand struct {
	Subject string
	PlanID  string
}

type DeleteSubjectAssignmentCommand struct {
	Subject string
}

type AssignmentListQuery struct {
	Subject string
	PlanID  string
	Limit   int
}

type SubjectProgressQuery struct {
	Subject string
}

type CheckCommand struct {
	Subject  string
	Meter    string
	Quantity float64
}

type UsageEvent struct {
	Subject  string
	Meter    string
	Quantity float64
}

type ClaimCommand struct {
	LockTTL     time.Duration
	MaxAttempts int
}

type EvaluateCommand struct {
	Subject string
	Meter   string
}

type CompleteCommand struct {
	Subject string
	Meter   string
}

type FailCommand struct {
	Subject    string
	Meter      string
	RetryAfter time.Duration
	Error      string
}

type PlanResult struct {
	Plan   Plan
	Limits []PlanLimit
}

type PlanListResult struct {
	Items []PlanResult
}

type SubjectAssignmentResult struct {
	Assignment SubjectAssignment
}

type SubjectAssignmentListResult struct {
	Items []SubjectAssignment
}

type StateListResult struct {
	Items []EntitlementState
}

type EventListResult struct {
	Items      []EntitlementEvent
	NextCursor string
}

type ProgressItem struct {
	MeterName      string
	Period         Period
	State          OverageState
	Current        float64
	Limit          float64
	Remaining      float64
	Percent        float64
	WarningPercent float64
	From           time.Time
	To             time.Time
	Unit           string
	Aggregation    domainmeter.Aggregation
}

type SubjectProgressResult struct {
	Subject string
	Plan    PlanResult
	Items   []ProgressItem
}

type EntitlementCheckResult struct {
	Allowed   bool
	State     OverageState
	Subject   string
	MeterName string
	Quantity  float64
	Current   float64
	Limit     float64
	Remaining float64
	PlanID    string
	PlanName  string
	Period    Period
	From      time.Time
	To        time.Time
	Message   string
}

type CheckJobResult struct {
	Job CheckJob
}

type EvaluationResult struct {
	Subject string
	Meter   string
	State   *EntitlementState
	Event   *EntitlementEvent
	Skipped bool
	Message string
}

func (s *service) CreatePlan(ctx context.Context, cmd SavePlanCommand) (PlanResult, error) {
	now := s.now()
	plan, limits, err := s.normalizePlan(ctx, "", cmd.Name, cmd.Description, cmd.Limits, now)
	if err != nil {
		return PlanResult{}, err
	}

	err = s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		if _, err := s.repo.SavePlan(txCtx, plan); err != nil {
			return err
		}
		return s.repo.ReplacePlanLimits(txCtx, plan.ID, limits)
	})
	if err != nil {
		return PlanResult{}, err
	}

	return PlanResult{Plan: plan, Limits: limits}, nil
}

func (s *service) UpdatePlan(ctx context.Context, cmd UpdatePlanCommand) (PlanResult, error) {
	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return PlanResult{}, fmt.Errorf("%w: plan id is required", domain.ErrInvalidInput)
	}

	existing, err := s.findPlan(ctx, id)
	if err != nil {
		return PlanResult{}, err
	}

	now := s.now()
	plan, limits, err := s.normalizePlan(ctx, id, cmd.Name, cmd.Description, cmd.Limits, existing.Plan.CreatedAt)
	if err != nil {
		return PlanResult{}, err
	}
	plan.UpdatedAt = now
	for i := range limits {
		limits[i].CreatedAt = now
		limits[i].UpdatedAt = now
	}

	err = s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		if _, err := s.repo.SavePlan(txCtx, plan); err != nil {
			return err
		}
		return s.repo.ReplacePlanLimits(txCtx, id, limits)
	})
	if err != nil {
		return PlanResult{}, err
	}

	return PlanResult{Plan: plan, Limits: limits}, nil
}

func (s *service) DeletePlan(ctx context.Context, cmd DeletePlanCommand) error {
	id := strings.TrimSpace(cmd.ID)
	if id == "" {
		return fmt.Errorf("%w: plan id is required", domain.ErrInvalidInput)
	}
	assigned, err := s.repo.CountAssignmentsForPlan(ctx, id)
	if err != nil {
		return err
	}
	if assigned > 0 {
		return errors.Join(domain.ErrConflict, errors.New("plan has assigned subjects"))
	}
	return s.repo.DeletePlan(ctx, id)
}

func (s *service) GetPlan(ctx context.Context, query GetPlanQuery) (PlanResult, error) {
	return s.findPlan(ctx, query.ID)
}

func (s *service) ListPlans(ctx context.Context, query ListPlansQuery) (PlanListResult, error) {
	plans, err := s.repo.FindPlans(ctx, PlanQuery{Limit: normalizeLimit(query.Limit)})
	if err != nil {
		return PlanListResult{}, err
	}
	if len(plans) == 0 {
		return PlanListResult{Items: []PlanResult{}}, nil
	}
	limitsByPlan, err := s.limitsByPlan(ctx, plans)
	if err != nil {
		return PlanListResult{}, err
	}
	items := make([]PlanResult, 0, len(plans))
	for _, plan := range plans {
		items = append(items, PlanResult{Plan: plan, Limits: limitsByPlan[plan.ID]})
	}
	return PlanListResult{Items: items}, nil
}

func (s *service) AssignSubject(ctx context.Context, cmd AssignSubjectCommand) (SubjectAssignmentResult, error) {
	subject := strings.TrimSpace(cmd.Subject)
	planID := strings.TrimSpace(cmd.PlanID)
	if subject == "" {
		return SubjectAssignmentResult{}, fmt.Errorf("%w: subject is required", domain.ErrInvalidInput)
	}
	if planID == "" {
		return SubjectAssignmentResult{}, fmt.Errorf("%w: plan id is required", domain.ErrInvalidInput)
	}
	plan, err := s.findPlan(ctx, planID)
	if err != nil {
		return SubjectAssignmentResult{}, err
	}

	now := s.now()
	assignment := SubjectAssignment{
		Subject:    subject,
		PlanID:     plan.Plan.ID,
		PlanName:   plan.Plan.Name,
		AssignedAt: now,
		UpdatedAt:  now,
	}
	saved, err := s.repo.SaveSubjectAssignment(ctx, assignment)
	if err != nil {
		return SubjectAssignmentResult{}, err
	}
	return SubjectAssignmentResult{Assignment: saved}, nil
}

func (s *service) DeleteSubjectAssignment(ctx context.Context, cmd DeleteSubjectAssignmentCommand) error {
	subject := strings.TrimSpace(cmd.Subject)
	if subject == "" {
		return fmt.Errorf("%w: subject is required", domain.ErrInvalidInput)
	}
	return s.repo.DeleteSubjectAssignment(ctx, subject)
}

func (s *service) ListSubjectAssignments(ctx context.Context, query AssignmentListQuery) (SubjectAssignmentListResult, error) {
	assignments, err := s.repo.FindSubjectAssignments(ctx, AssignmentQuery{
		Subject: strings.TrimSpace(query.Subject),
		PlanID:  strings.TrimSpace(query.PlanID),
		Limit:   normalizeLimit(query.Limit),
	})
	if err != nil {
		return SubjectAssignmentListResult{}, err
	}
	return SubjectAssignmentListResult{Items: assignments}, nil
}

func (s *service) ListEntitlementStates(ctx context.Context, query StateListQuery) (StateListResult, error) {
	states, err := s.repo.FindEntitlementStates(ctx, StateListQuery{
		Subject:   strings.TrimSpace(query.Subject),
		MeterName: strings.TrimSpace(query.MeterName),
		State:     OverageState(strings.TrimSpace(string(query.State))),
		Limit:     normalizeLimit(query.Limit),
	})
	if err != nil {
		return StateListResult{}, err
	}
	return StateListResult{Items: states}, nil
}

func (s *service) ListEntitlementEvents(ctx context.Context, query EventListQuery) (EventListResult, error) {
	limit := normalizeLimit(query.Limit)
	cursor, err := page.Decode(query.Cursor)
	if err != nil {
		return EventListResult{}, err
	}
	if query.Cursor != "" && (cursor.Time.IsZero() || cursor.ID == "") {
		return EventListResult{}, domain.ErrInvalidInput
	}

	events, err := s.repo.FindEntitlementEvents(ctx, EventQuery{
		Subject:   strings.TrimSpace(query.Subject),
		MeterName: strings.TrimSpace(query.MeterName),
		PlanID:    strings.TrimSpace(query.PlanID),
		State:     OverageState(strings.TrimSpace(string(query.State))),
		Type:      EventType(strings.TrimSpace(string(query.Type))),
		CreatedAt: cursor.Time,
		ID:        cursor.ID,
		Limit:     limit + 1,
	})
	if err != nil {
		return EventListResult{}, err
	}

	nextCursor := ""
	if len(events) > limit {
		events = events[:limit]
		last := events[len(events)-1]
		nextCursor, err = page.Encode(page.Cursor{Time: last.CreatedAt, ID: last.ID})
		if err != nil {
			return EventListResult{}, err
		}
	}

	return EventListResult{Items: events, NextCursor: nextCursor}, nil
}

func (s *service) GetSubjectProgress(ctx context.Context, query SubjectProgressQuery) (SubjectProgressResult, error) {
	subject := strings.TrimSpace(query.Subject)
	if subject == "" {
		return SubjectProgressResult{}, fmt.Errorf("%w: subject is required", domain.ErrInvalidInput)
	}

	assignment, err := s.findSubjectAssignment(ctx, subject)
	if err != nil {
		return SubjectProgressResult{}, err
	}
	plan, err := s.findPlan(ctx, assignment.PlanID)
	if err != nil {
		return SubjectProgressResult{}, err
	}

	items := make([]ProgressItem, 0, len(plan.Limits))
	for _, limit := range plan.Limits {
		item, err := s.progressItem(ctx, subject, limit)
		if err != nil {
			return SubjectProgressResult{}, err
		}
		items = append(items, item)
	}
	return SubjectProgressResult{Subject: subject, Plan: plan, Items: items}, nil
}

func (s *service) Check(ctx context.Context, cmd CheckCommand) (EntitlementCheckResult, error) {
	subject := strings.TrimSpace(cmd.Subject)
	meterName := strings.TrimSpace(cmd.Meter)
	if subject == "" {
		return EntitlementCheckResult{}, fmt.Errorf("%w: subject is required", domain.ErrInvalidInput)
	}
	if meterName == "" {
		return EntitlementCheckResult{}, fmt.Errorf("%w: meter is required", domain.ErrInvalidInput)
	}
	quantity := cmd.Quantity
	if quantity == 0 {
		quantity = 1
	}
	if !isFinitePositive(quantity) {
		return EntitlementCheckResult{}, fmt.Errorf("%w: quantity must be greater than zero", domain.ErrInvalidInput)
	}

	assignment, err := s.findSubjectAssignment(ctx, subject)
	if errors.Is(err, domain.ErrNotFound) {
		return EntitlementCheckResult{
			Allowed:   false,
			State:     StateNoPlan,
			Subject:   subject,
			MeterName: meterName,
			Quantity:  quantity,
			Message:   "subject is not assigned to a plan",
		}, nil
	}
	if err != nil {
		return EntitlementCheckResult{}, err
	}
	plan, err := s.findPlan(ctx, assignment.PlanID)
	if err != nil {
		return EntitlementCheckResult{}, err
	}
	limit, ok := plan.limitForMeter(meterName)
	if !ok {
		return EntitlementCheckResult{
			Allowed:   false,
			State:     StateNotInPlan,
			Subject:   subject,
			MeterName: meterName,
			Quantity:  quantity,
			PlanID:    plan.Plan.ID,
			PlanName:  plan.Plan.Name,
			Message:   "meter is not included in the subject's plan",
		}, nil
	}

	item, err := s.progressItem(ctx, subject, limit)
	if err != nil {
		return EntitlementCheckResult{}, err
	}
	projected := item.Current + quantity
	allowed := projected <= item.Limit
	remaining := item.Limit - projected
	if remaining < 0 {
		remaining = 0
	}
	state := item.State
	message := "quota is available"
	if !allowed {
		state = StateExceeded
		message = "quota would be exceeded"
	}

	return EntitlementCheckResult{
		Allowed:   allowed,
		State:     state,
		Subject:   subject,
		MeterName: meterName,
		Quantity:  quantity,
		Current:   item.Current,
		Limit:     item.Limit,
		Remaining: remaining,
		PlanID:    plan.Plan.ID,
		PlanName:  plan.Plan.Name,
		Period:    item.Period,
		From:      item.From,
		To:        item.To,
		Message:   message,
	}, nil
}

func (s *service) EnqueueForUsageEvents(ctx context.Context, events []UsageEvent) error {
	if len(events) == 0 {
		return nil
	}
	now := s.now()
	runAfter := now.Add(DefaultCheckEvaluationWait)
	seen := map[string]struct{}{}
	for _, event := range events {
		subject := strings.TrimSpace(event.Subject)
		meterName := strings.TrimSpace(event.Meter)
		if subject == "" || meterName == "" {
			continue
		}
		key := subject + "\x00" + meterName
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if err := s.repo.EnqueueEntitlementCheckJob(ctx, CheckJob{
			Subject:   subject,
			MeterName: meterName,
			RunAfter:  runAfter,
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) ClaimCheckJob(ctx context.Context, cmd ClaimCommand) (CheckJobResult, bool, error) {
	if cmd.LockTTL <= 0 {
		return CheckJobResult{}, false, fmt.Errorf("%w: lock ttl must be greater than zero", domain.ErrInvalidInput)
	}
	if cmd.MaxAttempts <= 0 {
		return CheckJobResult{}, false, fmt.Errorf("%w: max attempts must be greater than zero", domain.ErrInvalidInput)
	}
	job, ok, err := s.repo.ClaimEntitlementCheckJob(ctx, cmd)
	if err != nil || !ok {
		return CheckJobResult{}, ok, err
	}
	return CheckJobResult{Job: job}, true, nil
}

func (s *service) Evaluate(ctx context.Context, cmd EvaluateCommand) (EvaluationResult, error) {
	subject := strings.TrimSpace(cmd.Subject)
	meterName := strings.TrimSpace(cmd.Meter)
	if subject == "" {
		return EvaluationResult{}, fmt.Errorf("%w: subject is required", domain.ErrInvalidInput)
	}
	if meterName == "" {
		return EvaluationResult{}, fmt.Errorf("%w: meter is required", domain.ErrInvalidInput)
	}

	assignment, err := s.findSubjectAssignment(ctx, subject)
	if errors.Is(err, domain.ErrNotFound) {
		return EvaluationResult{Subject: subject, Meter: meterName, Skipped: true, Message: "subject is not assigned to a plan"}, nil
	}
	if err != nil {
		return EvaluationResult{}, err
	}

	plan, err := s.findPlan(ctx, assignment.PlanID)
	if err != nil {
		return EvaluationResult{}, err
	}
	limit, ok := plan.limitForMeter(meterName)
	if !ok {
		return EvaluationResult{Subject: subject, Meter: meterName, Skipped: true, Message: "meter is not included in the subject's plan"}, nil
	}

	item, err := s.progressItem(ctx, subject, limit)
	if err != nil {
		return EvaluationResult{}, err
	}
	now := s.now()
	state := entitlementStateFromProgress(subject, plan.Plan, item, now)
	previous, err := s.repo.GetEntitlementState(ctx, StateQuery{
		Subject:   subject,
		MeterName: item.MeterName,
		PlanID:    plan.Plan.ID,
		Period:    item.Period,
	})
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return EvaluationResult{}, err
	}

	var event *EntitlementEvent
	if eventType, ok := entitlementEventType(previous, state.State); ok {
		eventValue := EntitlementEvent{
			ID:             uuid.NewString(),
			Subject:        subject,
			MeterName:      item.MeterName,
			PlanID:         plan.Plan.ID,
			PlanName:       plan.Plan.Name,
			Period:         item.Period,
			PreviousState:  previous.State,
			State:          state.State,
			Type:           eventType,
			Current:        state.Current,
			Limit:          state.Limit,
			Remaining:      state.Remaining,
			WarningPercent: state.WarningPercent,
			Message:        state.Message,
			CreatedAt:      now,
		}
		event = &eventValue
	}

	err = s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repo.SaveEntitlementState(txCtx, state); err != nil {
			return err
		}
		if event != nil {
			return s.repo.SaveEntitlementEvent(txCtx, *event)
		}
		return nil
	})
	if err != nil {
		return EvaluationResult{}, err
	}

	return EvaluationResult{
		Subject: subject,
		Meter:   meterName,
		State:   &state,
		Event:   event,
	}, nil
}

func (s *service) CompleteCheckJob(ctx context.Context, cmd CompleteCommand) error {
	cmd.Subject = strings.TrimSpace(cmd.Subject)
	cmd.Meter = strings.TrimSpace(cmd.Meter)
	if cmd.Subject == "" {
		return fmt.Errorf("%w: subject is required", domain.ErrInvalidInput)
	}
	if cmd.Meter == "" {
		return fmt.Errorf("%w: meter is required", domain.ErrInvalidInput)
	}
	return s.repo.DeleteEntitlementCheckJob(ctx, cmd)
}

func (s *service) FailCheckJob(ctx context.Context, cmd FailCommand) error {
	cmd.Subject = strings.TrimSpace(cmd.Subject)
	cmd.Meter = strings.TrimSpace(cmd.Meter)
	if cmd.Subject == "" {
		return fmt.Errorf("%w: subject is required", domain.ErrInvalidInput)
	}
	if cmd.Meter == "" {
		return fmt.Errorf("%w: meter is required", domain.ErrInvalidInput)
	}
	if cmd.RetryAfter <= 0 {
		return fmt.Errorf("%w: retry after must be greater than zero", domain.ErrInvalidInput)
	}
	return s.repo.RequeueEntitlementCheckJob(ctx, cmd)
}

func (s *service) normalizePlan(ctx context.Context, id, name, description string, input []LimitCommand, createdAt time.Time) (Plan, []PlanLimit, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if id == "" {
		id = uuid.NewString()
	}
	if name == "" {
		return Plan{}, nil, fmt.Errorf("%w: plan name is required", domain.ErrInvalidInput)
	}
	if utf8.RuneCountInString(name) > MaxNameRunes {
		return Plan{}, nil, fmt.Errorf("%w: plan name cannot exceed %d characters", domain.ErrInvalidInput, MaxNameRunes)
	}
	if createdAt.IsZero() {
		createdAt = s.now()
	}
	if len(input) == 0 {
		return Plan{}, nil, fmt.Errorf("%w: at least one plan limit is required", domain.ErrInvalidInput)
	}

	now := s.now()
	plan := Plan{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   createdAt.UTC(),
		UpdatedAt:   now,
	}

	seen := map[string]struct{}{}
	limits := make([]PlanLimit, 0, len(input))
	for _, command := range input {
		limit, err := s.normalizeLimitCommand(ctx, plan.ID, command, now)
		if err != nil {
			return Plan{}, nil, err
		}
		key := limit.MeterName + "\x00" + string(limit.Period)
		if _, exists := seen[key]; exists {
			return Plan{}, nil, fmt.Errorf("%w: duplicate limit for meter %q and period %q", domain.ErrInvalidInput, limit.MeterName, limit.Period)
		}
		seen[key] = struct{}{}
		limits = append(limits, limit)
	}

	return plan, limits, nil
}

func (s *service) normalizeLimitCommand(ctx context.Context, planID string, command LimitCommand, now time.Time) (PlanLimit, error) {
	meterName := strings.TrimSpace(command.MeterName)
	if meterName == "" {
		return PlanLimit{}, fmt.Errorf("%w: meter is required", domain.ErrInvalidInput)
	}
	meters, err := s.meterRepo.Find(ctx, domainmeter.Query{Name: meterName, Limit: 1})
	if err != nil {
		return PlanLimit{}, err
	}
	if len(meters) == 0 {
		return PlanLimit{}, fmt.Errorf("%w: meter %q was not found", domain.ErrNotFound, meterName)
	}
	period, err := normalizePeriod(command.Period)
	if err != nil {
		return PlanLimit{}, err
	}
	if !isFinitePositive(command.Limit) {
		return PlanLimit{}, fmt.Errorf("%w: limit must be greater than zero", domain.ErrInvalidInput)
	}
	warning := command.WarningPercent
	if warning == 0 {
		warning = DefaultWarningPercent
	}
	if !isFinitePositive(warning) || warning > 100 {
		return PlanLimit{}, fmt.Errorf("%w: warning percent must be greater than zero and at most 100", domain.ErrInvalidInput)
	}

	return PlanLimit{
		ID:             uuid.NewString(),
		PlanID:         planID,
		MeterName:      meterName,
		Period:         period,
		Limit:          command.Limit,
		WarningPercent: warning,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func (s *service) findPlan(ctx context.Context, id string) (PlanResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return PlanResult{}, fmt.Errorf("%w: plan id is required", domain.ErrInvalidInput)
	}
	plans, err := s.repo.FindPlans(ctx, PlanQuery{ID: id, Limit: 1})
	if err != nil {
		return PlanResult{}, err
	}
	if len(plans) == 0 {
		return PlanResult{}, domain.ErrNotFound
	}
	limits, err := s.repo.FindPlanLimits(ctx, id)
	if err != nil {
		return PlanResult{}, err
	}
	return PlanResult{Plan: plans[0], Limits: limits}, nil
}

func (s *service) findSubjectAssignment(ctx context.Context, subject string) (SubjectAssignment, error) {
	assignments, err := s.repo.FindSubjectAssignments(ctx, AssignmentQuery{Subject: subject, Limit: 1})
	if err != nil {
		return SubjectAssignment{}, err
	}
	if len(assignments) == 0 {
		return SubjectAssignment{}, domain.ErrNotFound
	}
	return assignments[0], nil
}

func (s *service) limitsByPlan(ctx context.Context, plans []Plan) (map[string][]PlanLimit, error) {
	result := make(map[string][]PlanLimit, len(plans))
	for _, plan := range plans {
		limits, err := s.repo.FindPlanLimits(ctx, plan.ID)
		if err != nil {
			return nil, err
		}
		result[plan.ID] = limits
	}
	return result, nil
}

func (s *service) progressItem(ctx context.Context, subject string, limit PlanLimit) (ProgressItem, error) {
	meters, err := s.meterRepo.Find(ctx, domainmeter.Query{Name: limit.MeterName, Limit: 1})
	if err != nil {
		return ProgressItem{}, err
	}
	if len(meters) == 0 {
		return ProgressItem{}, domain.ErrNotFound
	}
	meter := meters[0]
	from, to := periodWindow(s.now(), limit.Period)
	counter, err := s.repo.GetEntitlementUsageCounter(ctx, CounterQuery{
		Subject:   subject,
		MeterName: meter.Name(),
		Period:    limit.Period,
		From:      from,
	})
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return ProgressItem{}, err
	}

	current := counterValue(counter, meter.Aggregation(), to.Sub(from).Seconds())
	remaining := limit.Limit - current
	if remaining < 0 {
		remaining = 0
	}
	percent := 0.0
	if limit.Limit > 0 {
		percent = current / limit.Limit * 100
	}

	return ProgressItem{
		MeterName:      meter.Name(),
		Period:         limit.Period,
		State:          overageState(percent, current, limit.Limit, limit.WarningPercent),
		Current:        current,
		Limit:          limit.Limit,
		Remaining:      remaining,
		Percent:        percent,
		WarningPercent: limit.WarningPercent,
		From:           from,
		To:             to,
		Unit:           meter.Unit(),
		Aggregation:    meter.Aggregation(),
	}, nil
}

func (p PlanResult) limitForMeter(meterName string) (PlanLimit, bool) {
	for _, limit := range p.Limits {
		if limit.MeterName == meterName {
			return limit, true
		}
	}
	return PlanLimit{}, false
}

func counterValue(counter EntitlementUsageCounter, aggregation domainmeter.Aggregation, durationSeconds float64) float64 {
	if counter.EventCount <= 0 {
		return 0
	}
	switch aggregation {
	case domainmeter.AggregationCount:
		return float64(counter.EventCount)
	case domainmeter.AggregationAverage:
		return counter.QuantitySum / float64(counter.EventCount)
	case domainmeter.AggregationMinimum:
		return counter.QuantityMin
	case domainmeter.AggregationMaximum:
		return counter.QuantityMax
	case domainmeter.AggregationFirst:
		return counter.FirstQuantity
	case domainmeter.AggregationLast:
		return counter.LastQuantity
	case domainmeter.AggregationRate:
		if durationSeconds <= 0 {
			return 0
		}
		return float64(counter.EventCount) / durationSeconds
	default:
		return counter.QuantitySum
	}
}

func normalizePeriod(value string) (Period, error) {
	switch Period(strings.ToLower(strings.TrimSpace(value))) {
	case "":
		return PeriodMonth, nil
	case PeriodDay:
		return PeriodDay, nil
	case PeriodWeek:
		return PeriodWeek, nil
	case PeriodMonth:
		return PeriodMonth, nil
	case PeriodYear:
		return PeriodYear, nil
	default:
		return "", fmt.Errorf("%w: unsupported plan period %q", domain.ErrInvalidInput, value)
	}
}

func periodWindow(now time.Time, period Period) (time.Time, time.Time) {
	now = now.UTC()
	switch period {
	case PeriodDay:
		from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		return from, from.AddDate(0, 0, 1)
	case PeriodWeek:
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(weekday - 1))
		return from, from.AddDate(0, 0, 7)
	case PeriodYear:
		from := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
		return from, from.AddDate(1, 0, 0)
	default:
		from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		return from, from.AddDate(0, 1, 0)
	}
}

func overageState(percent float64, current float64, limit float64, warningPercent float64) OverageState {
	if current >= limit {
		return StateExceeded
	}
	if percent >= warningPercent {
		return StateWarning
	}
	return StateOK
}

func entitlementStateFromProgress(subject string, plan Plan, item ProgressItem, now time.Time) EntitlementState {
	return EntitlementState{
		Subject:        subject,
		MeterName:      item.MeterName,
		PlanID:         plan.ID,
		PlanName:       plan.Name,
		Period:         item.Period,
		State:          item.State,
		Current:        item.Current,
		Limit:          item.Limit,
		Remaining:      item.Remaining,
		WarningPercent: item.WarningPercent,
		Message:        entitlementMessage(item.State),
		EvaluatedAt:    now,
		UpdatedAt:      now,
	}
}

func entitlementEventType(previous EntitlementState, next OverageState) (EventType, bool) {
	if previous.State == next {
		return "", false
	}
	switch next {
	case StateOK:
		if previous.State == "" {
			return "", false
		}
		return EventRecovered, true
	case StateWarning:
		return EventWarning, true
	case StateExceeded:
		return EventExceeded, true
	default:
		return "", false
	}
}

func entitlementMessage(state OverageState) string {
	switch state {
	case StateExceeded:
		return "quota exceeded"
	case StateWarning:
		return "quota warning threshold reached"
	default:
		return "quota is available"
	}
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}

func isFinitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
