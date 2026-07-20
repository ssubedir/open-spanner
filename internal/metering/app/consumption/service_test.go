package consumption

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	appentitlement "github.com/ssubedir/open-spanner/internal/metering/app/entitlement"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainconsumption "github.com/ssubedir/open-spanner/internal/metering/domain/consumption"
)

type memoryRepository struct {
	mu      sync.Mutex
	results map[string]domainconsumption.Decision
}

func (r *memoryRepository) CountExpired(context.Context, time.Time) (int, error) {
	return len(r.results), nil
}
func (r *memoryRepository) PruneExpired(context.Context, time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := len(r.results)
	r.results = map[string]domainconsumption.Decision{}
	return count, nil
}
func (r *memoryRepository) SavePruneRun(context.Context, string, time.Time, bool, int, time.Time) error {
	return nil
}

func (r *memoryRepository) Find(_ context.Context, key string) (domainconsumption.Decision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result, ok := r.results[key]
	if !ok {
		return domainconsumption.Decision{}, domain.ErrNotFound
	}
	return result, nil
}

func (r *memoryRepository) Save(_ context.Context, key string, result []byte) (domainconsumption.Decision, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.results[key]; ok {
		return existing, false, nil
	}
	stored, err := domainconsumption.FromSnapshot(key, result, time.Now().UTC())
	if err != nil {
		return domainconsumption.Decision{}, false, err
	}
	r.results[key] = stored
	return stored, true, nil
}

func (r *memoryRepository) List(_ context.Context, _ domainconsumption.Query) ([]domainconsumption.Decision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]domainconsumption.Decision, 0, len(r.results))
	for _, item := range r.results {
		items = append(items, item)
	}
	return items, nil
}

type usageStub struct {
	mu      sync.Mutex
	creates int
	events  map[string]appusage.Result
}

func (s *usageStub) Create(_ context.Context, cmd appusage.CreateCommand) (appusage.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.events[cmd.IdempotencyKey]; ok {
		return existing, nil
	}
	s.creates++
	result := appusage.Result{
		ID: "event-1", IdempotencyKey: cmd.IdempotencyKey, Subject: cmd.Subject,
		MeterName: cmd.MeterName, Quantity: cmd.Quantity, EventTime: cmd.EventTime,
	}
	s.events[cmd.IdempotencyKey] = result
	return result, nil
}

func (s *usageStub) GetByIdempotencyKey(_ context.Context, key string) (appusage.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.events[key]
	if !ok {
		return appusage.Result{}, domain.ErrNotFound
	}
	return result, nil
}

type entitlementStub struct {
	assessment appentitlement.ConsumptionAssessment
	err        error
	calls      int
}

func (s *entitlementStub) AssessConsumption(context.Context, appentitlement.ConsumptionAssessmentCommand) (appentitlement.ConsumptionAssessment, error) {
	s.calls++
	return s.assessment, s.err
}

type directTransactor struct{}

func (directTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func newTestService(assessment appentitlement.ConsumptionAssessment, assessmentErr error) (*service, *memoryRepository, *usageStub, *entitlementStub) {
	repo := &memoryRepository{results: map[string]domainconsumption.Decision{}}
	usage := &usageStub{events: map[string]appusage.Result{}}
	entitlements := &entitlementStub{assessment: assessment, err: assessmentErr}
	return NewService(repo, usage, entitlements, directTransactor{}).(*service), repo, usage, entitlements
}

func testCommand(key string) Command {
	return Command{
		IdempotencyKey: key, Subject: "org_123", MeterName: "api_calls", Quantity: 1,
		EventTime: time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC),
	}
}

func TestRejectedDecisionReplaysWithoutReevaluation(t *testing.T) {
	assessment := appentitlement.ConsumptionAssessment{
		Allowed: false, State: appentitlement.StateExceeded, Enforcement: appentitlement.EnforcementHard,
		FailurePolicy: appentitlement.FailurePolicyFailOpen, Current: 10, Projected: 11, Limit: 10,
	}
	service, _, usage, entitlements := newTestService(assessment, nil)

	first, err := service.Consume(context.Background(), testCommand("reject-1"))
	if err != nil || first.Accepted || first.Replayed {
		t.Fatalf("first rejection = %#v, %v", first, err)
	}
	entitlements.assessment.Allowed = true
	second, err := service.Consume(context.Background(), testCommand("reject-1"))
	if err != nil || second.Accepted || !second.Replayed {
		t.Fatalf("replayed rejection = %#v, %v", second, err)
	}
	if second.Quota.Current != 10 || second.Quota.Projected != 11 || entitlements.calls != 1 || usage.creates != 0 {
		t.Fatalf("replayed rejection changed: result=%#v calls=%d creates=%d", second, entitlements.calls, usage.creates)
	}
}

func TestFailOpenDecisionPersistsAcceptedUsage(t *testing.T) {
	assessment := appentitlement.ConsumptionAssessment{FailurePolicy: appentitlement.FailurePolicyFailOpen}
	service, _, usage, entitlements := newTestService(assessment, errors.New("counter unavailable"))

	first, err := service.Consume(context.Background(), testCommand("open-1"))
	if err != nil || !first.Accepted || !first.EvaluationFailed || first.Replayed {
		t.Fatalf("fail-open result = %#v, %v", first, err)
	}
	entitlements.err = nil
	second, err := service.Consume(context.Background(), testCommand("open-1"))
	if err != nil || !second.Accepted || !second.EvaluationFailed || !second.Replayed {
		t.Fatalf("fail-open replay = %#v, %v", second, err)
	}
	if usage.creates != 1 || entitlements.calls != 1 {
		t.Fatalf("fail-open replay calls=%d creates=%d, want 1/1", entitlements.calls, usage.creates)
	}
}

func TestFailClosedDecisionPersistsRejection(t *testing.T) {
	assessment := appentitlement.ConsumptionAssessment{FailurePolicy: appentitlement.FailurePolicyFailClosed}
	service, _, usage, entitlements := newTestService(assessment, errors.New("counter unavailable"))

	first, err := service.Consume(context.Background(), testCommand("closed-1"))
	if err != nil || first.Accepted || !first.EvaluationFailed || first.Replayed {
		t.Fatalf("fail-closed result = %#v, %v", first, err)
	}
	entitlements.err = nil
	second, err := service.Consume(context.Background(), testCommand("closed-1"))
	if err != nil || second.Accepted || !second.EvaluationFailed || !second.Replayed {
		t.Fatalf("fail-closed replay = %#v, %v", second, err)
	}
	if usage.creates != 0 || entitlements.calls != 1 {
		t.Fatalf("fail-closed replay calls=%d creates=%d, want 1/0", entitlements.calls, usage.creates)
	}
}

func TestPrunedRejectedDecisionCanBeEvaluatedAsNewAttempt(t *testing.T) {
	assessment := appentitlement.ConsumptionAssessment{
		Allowed: false, State: appentitlement.StateExceeded, Enforcement: appentitlement.EnforcementHard,
		FailurePolicy: appentitlement.FailurePolicyFailOpen,
	}
	service, _, usage, entitlements := newTestService(assessment, nil)
	ctx := context.Background()
	if first, err := service.Consume(ctx, testCommand("expired-1")); err != nil || first.Accepted {
		t.Fatalf("initial decision = %#v, %v", first, err)
	}
	pruned, err := service.PruneDecisions(ctx, PruneCommand{Before: time.Now().UTC().Add(time.Hour)})
	if err != nil || pruned.Deleted != 1 || pruned.DryRun {
		t.Fatalf("prune result = %#v, %v", pruned, err)
	}
	entitlements.assessment.Allowed = true
	second, err := service.Consume(ctx, testCommand("expired-1"))
	if err != nil || !second.Accepted || second.Replayed {
		t.Fatalf("new decision = %#v, %v", second, err)
	}
	if entitlements.calls != 2 || usage.creates != 1 {
		t.Fatalf("new attempt calls=%d creates=%d, want 2/1", entitlements.calls, usage.creates)
	}
}
