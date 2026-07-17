package system

import (
	"context"
	"fmt"
	"math"
	"time"

	apptransaction "github.com/ssubedir/open-spanner/internal/metering/app/transaction"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type Service interface {
	Stats(ctx context.Context) (StatsResult, error)
	RecordWorkerHeartbeat(ctx context.Context, workerName, instanceID string, startedAt, heartbeatAt time.Time) error
	RemoveWorkerHeartbeat(ctx context.Context, workerName, instanceID string) error
	Reconcile(ctx context.Context, query ReconciliationQuery) (ReconciliationResult, error)
	RepairCounter(ctx context.Context, cmd RepairCounterCommand) (CounterRepairResult, error)
	ListCounterRepairRuns(ctx context.Context, limit int) ([]CounterRepairResult, error)
	ClaimScheduledReconciliation(ctx context.Context, now, lockedUntil time.Time) (ReconciliationClaim, bool, error)
	RunScheduledReconciliation(ctx context.Context, claim ReconciliationClaim, scheduleInterval time.Duration, query ReconciliationQuery) (ReconciliationRun, bool, error)
	FailScheduledReconciliation(ctx context.Context, claim ReconciliationClaim, retryAt time.Time, runErr error) error
	MarkReconciliationNotified(ctx context.Context, workspaceID, fingerprint string) error
	ListReconciliationRuns(ctx context.Context, limit int) ([]ReconciliationRun, error)
	ClaimReconciliationNotification(ctx context.Context, now, lockedUntil time.Time) (ReconciliationNotification, bool, error)
	CompleteReconciliationNotification(ctx context.Context, notification ReconciliationNotification) error
	RetryReconciliationNotification(ctx context.Context, notification ReconciliationNotification, nextAttemptAt time.Time, maxAttempts int, deliveryErr error) error
	ListReconciliationNotifications(ctx context.Context, limit int) ([]ReconciliationNotification, error)
	RequeueReconciliationNotification(ctx context.Context, id string) error
	ListWorkerDeadLetters(ctx context.Context, limit int) ([]WorkerDeadLetter, error)
	RetryWorkerDeadLetter(ctx context.Context, id string) error
	PruneOperationalHistory(ctx context.Context, before time.Time, batchSize int) (OperationalHistoryPruneResult, error)
	ClaimMaintenanceLease(ctx context.Context, workerName string, now, lockedUntil time.Time) (MaintenanceLease, bool, error)
	ReleaseMaintenanceLease(ctx context.Context, lease MaintenanceLease) error
}

type OperationalHistoryRepository interface {
	PruneOperationalHistory(ctx context.Context, before time.Time, batchSize int) (OperationalHistoryPruneResult, error)
}

type service struct {
	repo             Repository
	transactor       apptransaction.Transactor
	staleAfter       time.Duration
	rollupStaleAfter time.Duration
	workerEnabled    map[string]bool
}

type Repository interface {
	FindStats(ctx context.Context) (StatsResult, error)
	ListDecisionReconciliationRows(ctx context.Context, since time.Time, limit int) ([]DecisionReconciliationRow, error)
	ListActiveEntitlementCounters(ctx context.Context, now time.Time, limit int) ([]CounterReconciliationRow, error)
	ListCounterReconciliationEvents(ctx context.Context, counter CounterReconciliationRow) ([]ReconciliationEvent, error)
	ListCounterReconciliationAssignments(ctx context.Context, counter CounterReconciliationRow) ([]ReconciliationAssignment, error)
	GetEntitlementCounterForRepair(ctx context.Context, target CounterRepairTarget) (CounterReconciliationRow, error)
	UpdateEntitlementCounterForRepair(ctx context.Context, counter CounterReconciliationRow, expectedUpdatedAt time.Time, replacement CounterSnapshot, updatedAt time.Time) (bool, error)
	SaveQuotaCounterRepairRun(ctx context.Context, run CounterRepairResult) error
	ListQuotaCounterRepairRuns(ctx context.Context, limit int) ([]CounterRepairResult, error)
	FindLatestMeterPruneCutoff(ctx context.Context, meterName string) (time.Time, error)
	ClaimReconciliationSchedule(ctx context.Context, now, lockedUntil time.Time) (ReconciliationClaim, bool, error)
	SaveReconciliationRun(ctx context.Context, workspaceID string, run ReconciliationRun) error
	CompleteReconciliationSchedule(ctx context.Context, claim ReconciliationClaim, fingerprint string, nextRunAt time.Time) error
	FailReconciliationSchedule(ctx context.Context, claim ReconciliationClaim, failureFingerprint string, nextRunAt time.Time) error
	ClaimMaintenanceLease(ctx context.Context, workerName, claimToken string, now, lockedUntil time.Time) (MaintenanceLease, bool, error)
	ReleaseMaintenanceLease(ctx context.Context, lease MaintenanceLease) error
	MarkReconciliationNotified(ctx context.Context, workspaceID, fingerprint string) error
	ListReconciliationRuns(ctx context.Context, limit int) ([]ReconciliationRun, error)
	GetReconciliationSchedule(ctx context.Context) (ReconciliationSchedule, bool, error)
	SaveReconciliationNotification(ctx context.Context, notification ReconciliationNotification) error
	ClaimReconciliationNotification(ctx context.Context, now, lockedUntil time.Time) (ReconciliationNotification, bool, error)
	CompleteReconciliationNotification(ctx context.Context, notification ReconciliationNotification) error
	RetryReconciliationNotification(ctx context.Context, notification ReconciliationNotification, nextAttemptAt time.Time, maxAttempts int, deliveryErr error) error
	ListReconciliationNotifications(ctx context.Context, limit int) ([]ReconciliationNotification, error)
	CountReconciliationNotifications(ctx context.Context) (ReconciliationNotificationCounts, error)
	RequeueReconciliationNotification(ctx context.Context, id string, nextAttemptAt time.Time) (bool, error)
	SaveReconciliationNotificationAttempt(ctx context.Context, attempt ReconciliationNotificationAttempt) error
	ListReconciliationNotificationAttempts(ctx context.Context, notificationID string) ([]ReconciliationNotificationAttempt, error)
	UpsertWorkerHeartbeat(ctx context.Context, heartbeat WorkerHeartbeat) error
	DeleteWorkerHeartbeat(ctx context.Context, workerName, instanceID string) error
	ListWorkerHeartbeats(ctx context.Context) ([]WorkerHeartbeat, error)
	ListWorkerDiagnostics(ctx context.Context, now time.Time) ([]WorkerDiagnostics, error)
	ListWorkerDeadLetters(ctx context.Context, limit int) ([]WorkerDeadLetter, error)
	GetWorkerDeadLetter(ctx context.Context, id string) (WorkerDeadLetter, error)
	EnqueueWorkerDeadLetter(ctx context.Context, deadLetter WorkerDeadLetter, now time.Time) (bool, error)
	MarkWorkerDeadLetterRequeued(ctx context.Context, id string, now time.Time) (bool, error)
}

const (
	defaultReconciliationLimit         = 100
	maxReconciliationLimit             = 500
	defaultReconciliationLookbackHours = 24
	maxReconciliationLookbackHours     = 720
)

type ReconciliationQuery struct {
	Limit         int
	LookbackHours int
}

type ReconciliationResult struct {
	Status          string
	DecisionChecked int
	CountersChecked int
	Issues          []ReconciliationIssue
	Truncated       bool
	LookbackHours   int
	CheckedAt       time.Time
}

type ReconciliationIssue struct {
	Kind           string    `json:"kind"`
	Severity       string    `json:"severity"`
	Subject        string    `json:"subject,omitempty"`
	MeterName      string    `json:"meter,omitempty"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	Period         string    `json:"period,omitempty"`
	PeriodStart    time.Time `json:"period_start,omitempty"`
	Expected       string    `json:"expected"`
	Actual         string    `json:"actual"`
	Message        string    `json:"message"`
}

type DecisionReconciliationRow struct {
	IdempotencyKey string
	Subject        string
	MeterName      string
	Accepted       bool
	CreatedAt      time.Time
	EventID        string
	EventSubject   string
	EventMeterName string
}

type CounterReconciliationRow struct {
	Subject            string
	MeterName          string
	Period             string
	PeriodStart        time.Time
	PeriodEnd          time.Time
	EventCount         int64
	QuantitySum        float64
	QuantityMin        float64
	QuantityMax        float64
	UpdatedAt          time.Time
	EventRetentionDays int
	FirstQuantity      float64
	FirstEventTime     time.Time
	LastQuantity       float64
	LastEventTime      time.Time
}

type ReconciliationEvent struct {
	ID         string
	Quantity   float64
	EventTime  time.Time
	ReceivedAt time.Time
}

type ReconciliationAssignment struct {
	ID             string
	AssignedAt     time.Time
	PeriodAnchorAt time.Time
	UnassignedAt   time.Time
}

type StatsResult struct {
	Meters                int
	UsageEvents           int
	PruneRuns             int
	LastPruneRun          LastPruneRunResult
	ExportCleanupRuns     int
	LastExportCleanupRun  LastExportCleanupRunResult
	ConsumptionDecisions  int
	DecisionPruneRuns     int
	LastDecisionPruneRun  LastDecisionPruneRunResult
	LastReconciliationRun ReconciliationRun
	ReconciliationHealth  ReconciliationHealth
	WorkerHealth          []WorkerHealth
	RollupHealth          RollupHealth
	IngestionSafety       IngestionSafety
}

type IngestionSafetyRepository interface {
	FindIngestionSafety(ctx context.Context) (IngestionSafety, error)
}
type IngestionSafety struct {
	AcceptedEvents  int64
	RejectedEvents  int64
	ThrottledEvents int64
}

type RollupCoverageRepository interface {
	ListRollupMeterCoverage(ctx context.Context) ([]RollupMeterCoverage, error)
}

type RollupMeterCoverage struct {
	MeterName          string
	RetentionDays      int
	FinalizedThrough   time.Time
	SourceEvents       int64
	RollupRows         int64
	LastRunAt          time.Time
	InvalidRollupRows  int
	RawBeforeFinalized int
}

type RollupHealth struct {
	Status           string
	Meters           int
	HealthyMeters    int
	Issues           int
	FinalizedThrough time.Time
	Items            []RollupMeterHealth
}

type RollupMeterHealth struct {
	MeterName        string
	Status           string
	ExpectedThrough  time.Time
	FinalizedThrough time.Time
	SourceEvents     int64
	RollupRows       int64
	LastRunAt        time.Time
	Issue            string
}

type WorkerHeartbeat struct {
	Name            string
	InstanceID      string
	StartedAt       time.Time
	LastHeartbeatAt time.Time
}

type WorkerHealth struct {
	Name            string
	Status          string
	StartedAt       time.Time
	LastHeartbeatAt time.Time
	PendingJobs     int
	RunningJobs     int
	FailedJobs      int
	OldestPendingAt time.Time
	LastSuccessAt   time.Time
	LastFailureAt   time.Time
	ReplicaCount    int
	HealthyReplicas int
	StaleReplicas   int
	Instances       []WorkerInstanceHealth
}

type WorkerInstanceHealth struct {
	InstanceID      string
	Status          string
	StartedAt       time.Time
	LastHeartbeatAt time.Time
}

type WorkerDiagnostics struct {
	Name            string
	PendingJobs     int
	RunningJobs     int
	FailedJobs      int
	OldestPendingAt time.Time
	LastSuccessAt   time.Time
	LastFailureAt   time.Time
}

type LastDecisionPruneRunResult struct {
	ID        string
	Before    time.Time
	Deleted   int
	DryRun    bool
	CreatedAt time.Time
}

type LastPruneRunResult struct {
	ID        string
	Deleted   int
	DryRun    bool
	CreatedAt time.Time
}

type LastExportCleanupRunResult struct {
	ID             string
	ExpiredBefore  time.Time
	FilesDeleted   int
	BytesReclaimed int64
	Failures       int
	CreatedAt      time.Time
}

type OperationalHistoryPruneResult struct {
	IngestionAudits             int
	UsagePruneRuns              int
	DecisionPruneRuns           int
	ExportCleanupRuns           int
	ExportJobs                  int
	AlertDeliveries             int
	AlertDeliveryJobs           int
	ReconciliationRuns          int
	ReconciliationNotifications int
	RollupRuns                  int
}

func (r OperationalHistoryPruneResult) Total() int {
	return r.IngestionAudits + r.UsagePruneRuns + r.DecisionPruneRuns + r.ExportCleanupRuns + r.ExportJobs + r.AlertDeliveries + r.AlertDeliveryJobs + r.ReconciliationRuns + r.ReconciliationNotifications + r.RollupRuns
}

type ServiceOptions struct {
	ReconciliationStaleAfter time.Duration
	RollupStaleAfter         time.Duration
	WorkerEnabled            map[string]bool
}

func NewService(repo Repository, transactor apptransaction.Transactor, options ...ServiceOptions) Service {
	if repo == nil || transactor == nil {
		panic("system service requires repository and transactor")
	}
	staleAfter := 30 * time.Minute
	rollupStaleAfter := 2 * time.Hour
	if len(options) > 0 && options[0].ReconciliationStaleAfter > 0 {
		staleAfter = options[0].ReconciliationStaleAfter
	}
	if len(options) > 0 && options[0].RollupStaleAfter > 0 {
		rollupStaleAfter = options[0].RollupStaleAfter
	}
	workerEnabled := map[string]bool{}
	if len(options) > 0 && options[0].WorkerEnabled != nil {
		workerEnabled = options[0].WorkerEnabled
	}
	return &service{repo: repo, transactor: transactor, staleAfter: staleAfter, rollupStaleAfter: rollupStaleAfter, workerEnabled: workerEnabled}
}

func (s *service) Stats(ctx context.Context) (StatsResult, error) {
	stats, err := s.repo.FindStats(ctx)
	if err != nil {
		return StatsResult{}, err
	}
	runs, err := s.repo.ListReconciliationRuns(ctx, 1)
	if err != nil {
		return StatsResult{}, err
	}
	if len(runs) > 0 {
		stats.LastReconciliationRun = runs[0]
	}
	schedule, exists, err := s.repo.GetReconciliationSchedule(ctx)
	if err != nil {
		return StatsResult{}, err
	}
	counts, err := s.repo.CountReconciliationNotifications(ctx)
	if err != nil {
		return StatsResult{}, err
	}
	stats.ReconciliationHealth = reconciliationHealth(schedule, exists, stats.LastReconciliationRun, counts, time.Now().UTC(), s.staleAfter)
	now := time.Now().UTC()
	heartbeats, err := s.repo.ListWorkerHeartbeats(ctx)
	if err != nil {
		return StatsResult{}, err
	}
	diagnostics, err := s.repo.ListWorkerDiagnostics(ctx, now)
	if err != nil {
		return StatsResult{}, err
	}
	stats.WorkerHealth = workerHealth(s.workerEnabled, heartbeats, diagnostics, now, 30*time.Second, 5*time.Minute)
	if repo, ok := s.repo.(RollupCoverageRepository); ok {
		coverage, err := repo.ListRollupMeterCoverage(ctx)
		if err != nil {
			return StatsResult{}, err
		}
		stats.RollupHealth = rollupHealth(coverage, now, s.rollupStaleAfter)
	}
	if repo, ok := s.repo.(IngestionSafetyRepository); ok {
		stats.IngestionSafety, err = repo.FindIngestionSafety(ctx)
		if err != nil {
			return StatsResult{}, err
		}
	}
	return stats, nil
}

func rollupHealth(coverage []RollupMeterCoverage, now time.Time, staleAfter time.Duration) RollupHealth {
	health := RollupHealth{Status: "healthy", Meters: len(coverage), Items: make([]RollupMeterHealth, 0, len(coverage))}
	notStarted := 0
	for _, row := range coverage {
		expected := now.AddDate(0, 0, -row.RetentionDays).UTC().Truncate(time.Hour)
		item := RollupMeterHealth{MeterName: row.MeterName, Status: "healthy", ExpectedThrough: expected, FinalizedThrough: row.FinalizedThrough, SourceEvents: row.SourceEvents, RollupRows: row.RollupRows, LastRunAt: row.LastRunAt}
		switch {
		case row.FinalizedThrough.IsZero():
			item.Status, item.Issue = "not_started", "No finalized rollup coverage has been recorded."
			notStarted++
		case row.InvalidRollupRows > 0:
			item.Status, item.Issue = "degraded", fmt.Sprintf("%d malformed rollup rows detected.", row.InvalidRollupRows)
		case row.RawBeforeFinalized > 0:
			item.Status, item.Issue = "degraded", fmt.Sprintf("%d raw events remain behind the finalized cutoff.", row.RawBeforeFinalized)
		case row.FinalizedThrough.Before(expected.Add(-staleAfter)):
			item.Status, item.Issue = "stale", "Finalized coverage is behind the meter retention cutoff."
		default:
			health.HealthyMeters++
		}
		if item.Status != "healthy" {
			health.Issues++
		}
		if !row.FinalizedThrough.IsZero() && (health.FinalizedThrough.IsZero() || row.FinalizedThrough.Before(health.FinalizedThrough)) {
			health.FinalizedThrough = row.FinalizedThrough
		}
		health.Items = append(health.Items, item)
	}
	if health.Issues > 0 {
		health.Status = "degraded"
		if notStarted == len(coverage) {
			health.Status = "not_started"
		} else {
			allStale := true
			for _, item := range health.Items {
				if item.Status != "healthy" && item.Status != "stale" {
					allStale = false
					break
				}
			}
			if allStale {
				health.Status = "stale"
			}
		}
	}
	return health
}

func (s *service) RecordWorkerHeartbeat(ctx context.Context, workerName, instanceID string, startedAt, heartbeatAt time.Time) error {
	if workerName == "" {
		return fmt.Errorf("%w: worker name is required", domain.ErrInvalidInput)
	}
	if instanceID == "" {
		return fmt.Errorf("%w: worker instance id is required", domain.ErrInvalidInput)
	}
	return s.repo.UpsertWorkerHeartbeat(ctx, WorkerHeartbeat{Name: workerName, InstanceID: instanceID, StartedAt: startedAt.UTC(), LastHeartbeatAt: heartbeatAt.UTC()})
}

func (s *service) RemoveWorkerHeartbeat(ctx context.Context, workerName, instanceID string) error {
	if workerName == "" || instanceID == "" {
		return fmt.Errorf("%w: worker name and instance id are required", domain.ErrInvalidInput)
	}
	return s.repo.DeleteWorkerHeartbeat(ctx, workerName, instanceID)
}

func (s *service) PruneOperationalHistory(ctx context.Context, before time.Time, batchSize int) (OperationalHistoryPruneResult, error) {
	if before.IsZero() {
		return OperationalHistoryPruneResult{}, fmt.Errorf("%w: operational history cutoff is required", domain.ErrInvalidInput)
	}
	if batchSize < 1 || batchSize > 10000 {
		return OperationalHistoryPruneResult{}, fmt.Errorf("%w: operational history batch size must be between 1 and 10000", domain.ErrInvalidInput)
	}
	var result OperationalHistoryPruneResult
	err := s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		var err error
		repo, ok := s.repo.(OperationalHistoryRepository)
		if !ok {
			return fmt.Errorf("operational history cleanup is not supported")
		}
		result, err = repo.PruneOperationalHistory(txCtx, before.UTC(), batchSize)
		return err
	})
	return result, err
}

func workerHealth(enabled map[string]bool, heartbeats []WorkerHeartbeat, diagnostics []WorkerDiagnostics, now time.Time, staleAfter, backlogStaleAfter time.Duration) []WorkerHealth {
	byName := map[string][]WorkerHeartbeat{}
	for _, heartbeat := range heartbeats {
		byName[heartbeat.Name] = append(byName[heartbeat.Name], heartbeat)
	}
	diagnosticsByName := map[string]WorkerDiagnostics{}
	for _, diagnostic := range diagnostics {
		diagnosticsByName[diagnostic.Name] = diagnostic
	}
	names := []string{"export", "alert", "entitlement", "retention", "history", "reconciliation"}
	result := make([]WorkerHealth, 0, len(names))
	for _, name := range names {
		item := WorkerHealth{Name: name, Status: "not_started"}
		if diagnostic, ok := diagnosticsByName[name]; ok {
			item.PendingJobs = diagnostic.PendingJobs
			item.RunningJobs = diagnostic.RunningJobs
			item.FailedJobs = diagnostic.FailedJobs
			item.OldestPendingAt = diagnostic.OldestPendingAt
			item.LastSuccessAt = diagnostic.LastSuccessAt
			item.LastFailureAt = diagnostic.LastFailureAt
		}
		if active, configured := enabled[name]; configured && !active {
			item.Status = "disabled"
			result = append(result, item)
			continue
		}
		if heartbeats := byName[name]; len(heartbeats) > 0 {
			item.ReplicaCount = len(heartbeats)
			item.Instances = make([]WorkerInstanceHealth, 0, len(heartbeats))
			for _, heartbeat := range heartbeats {
				instance := WorkerInstanceHealth{InstanceID: heartbeat.InstanceID, Status: "healthy", StartedAt: heartbeat.StartedAt, LastHeartbeatAt: heartbeat.LastHeartbeatAt}
				if now.Sub(heartbeat.LastHeartbeatAt) > staleAfter {
					instance.Status = "stale"
					item.StaleReplicas++
				} else {
					item.HealthyReplicas++
				}
				if item.StartedAt.IsZero() || heartbeat.StartedAt.Before(item.StartedAt) {
					item.StartedAt = heartbeat.StartedAt
				}
				if heartbeat.LastHeartbeatAt.After(item.LastHeartbeatAt) {
					item.LastHeartbeatAt = heartbeat.LastHeartbeatAt
				}
				item.Instances = append(item.Instances, instance)
			}
			item.Status = "healthy"
			if item.HealthyReplicas == 0 {
				item.Status = "stale"
			} else if item.StaleReplicas > 0 || item.FailedJobs > 0 || (!item.OldestPendingAt.IsZero() && now.Sub(item.OldestPendingAt) > backlogStaleAfter) {
				item.Status = "degraded"
			}
		}
		result = append(result, item)
	}
	return result
}

func (s *service) Reconcile(ctx context.Context, query ReconciliationQuery) (ReconciliationResult, error) {
	limit := query.Limit
	if limit == 0 {
		limit = defaultReconciliationLimit
	}
	if limit < 1 || limit > maxReconciliationLimit {
		return ReconciliationResult{}, fmt.Errorf("%w: limit must be between 1 and %d", domain.ErrInvalidInput, maxReconciliationLimit)
	}
	lookback := query.LookbackHours
	if lookback == 0 {
		lookback = defaultReconciliationLookbackHours
	}
	if lookback < 1 || lookback > maxReconciliationLookbackHours {
		return ReconciliationResult{}, fmt.Errorf("%w: lookback_hours must be between 1 and %d", domain.ErrInvalidInput, maxReconciliationLookbackHours)
	}

	now := time.Now().UTC()
	result := ReconciliationResult{Status: "healthy", Issues: []ReconciliationIssue{}, LookbackHours: lookback, CheckedAt: now}
	decisions, err := s.repo.ListDecisionReconciliationRows(ctx, now.Add(-time.Duration(lookback)*time.Hour), limit+1)
	if err != nil {
		return ReconciliationResult{}, err
	}
	if len(decisions) > limit {
		result.Truncated = true
		decisions = decisions[:limit]
	}
	result.DecisionChecked = len(decisions)
	for _, row := range decisions {
		result.Issues = append(result.Issues, decisionIssues(row)...)
	}

	counters, err := s.repo.ListActiveEntitlementCounters(ctx, now, limit+1)
	if err != nil {
		return ReconciliationResult{}, err
	}
	if len(counters) > limit {
		result.Truncated = true
		counters = counters[:limit]
	}
	pruneCutoffs := make(map[string]time.Time)
	for _, counter := range counters {
		pruneCutoff, ok := pruneCutoffs[counter.MeterName]
		if !ok {
			pruneCutoff, err = s.repo.FindLatestMeterPruneCutoff(ctx, counter.MeterName)
			if err != nil {
				return ReconciliationResult{}, err
			}
			pruneCutoffs[counter.MeterName] = pruneCutoff
		}
		if !counterSourceComplete(counter, now, pruneCutoff) {
			continue
		}
		result.CountersChecked++
		issues, err := s.reconcileCounter(ctx, counter)
		if err != nil {
			return ReconciliationResult{}, err
		}
		result.Issues = append(result.Issues, issues...)
	}
	if len(result.Issues) > 0 {
		result.Status = "drift_detected"
	}
	return result, nil
}

func decisionIssues(row DecisionReconciliationRow) []ReconciliationIssue {
	base := ReconciliationIssue{Severity: "critical", Subject: row.Subject, MeterName: row.MeterName, IdempotencyKey: row.IdempotencyKey}
	if row.Accepted && row.EventID == "" {
		base.Kind, base.Expected, base.Actual = "accepted_decision_missing_event", "usage event", "missing"
		base.Message = "An accepted consumption decision has no matching usage event."
		return []ReconciliationIssue{base}
	}
	if !row.Accepted && row.EventID != "" {
		base.Kind, base.Expected, base.Actual = "rejected_decision_has_event", "no usage event", row.EventID
		base.Message = "A rejected consumption decision has a matching usage event."
		return []ReconciliationIssue{base}
	}
	if row.EventID != "" && (row.Subject != row.EventSubject || row.MeterName != row.EventMeterName) {
		base.Kind = "decision_event_identity_mismatch"
		base.Expected = row.Subject + "/" + row.MeterName
		base.Actual = row.EventSubject + "/" + row.EventMeterName
		base.Message = "The decision and usage event disagree on subject or meter."
		return []ReconciliationIssue{base}
	}
	return nil
}

func (s *service) reconcileCounter(ctx context.Context, counter CounterReconciliationRow) ([]ReconciliationIssue, error) {
	matched, err := s.matchedCounterEvents(ctx, counter)
	if err != nil {
		return nil, err
	}
	expectedCount := int64(len(matched))
	expectedSum, expectedMin, expectedMax := counterAggregates(matched)
	base := ReconciliationIssue{Severity: "critical", Subject: counter.Subject, MeterName: counter.MeterName, Period: counter.Period, PeriodStart: counter.PeriodStart}
	issues := make([]ReconciliationIssue, 0, 4)
	if counter.EventCount != expectedCount {
		issue := base
		issue.Kind, issue.Expected, issue.Actual = "counter_event_count_mismatch", fmt.Sprint(expectedCount), fmt.Sprint(counter.EventCount)
		issue.Message = "The active quota counter event count differs from source usage."
		issues = append(issues, issue)
	}
	if !floatEqual(counter.QuantitySum, expectedSum) {
		issues = append(issues, counterValueIssue(base, "counter_quantity_sum_mismatch", expectedSum, counter.QuantitySum, "sum"))
	}
	if expectedCount > 0 && !floatEqual(counter.QuantityMin, expectedMin) {
		issues = append(issues, counterValueIssue(base, "counter_quantity_min_mismatch", expectedMin, counter.QuantityMin, "minimum"))
	}
	if expectedCount > 0 && !floatEqual(counter.QuantityMax, expectedMax) {
		issues = append(issues, counterValueIssue(base, "counter_quantity_max_mismatch", expectedMax, counter.QuantityMax, "maximum"))
	}
	return issues, nil
}

func (s *service) matchedCounterEvents(ctx context.Context, counter CounterReconciliationRow) ([]ReconciliationEvent, error) {
	events, err := s.repo.ListCounterReconciliationEvents(ctx, counter)
	if err != nil {
		return nil, err
	}
	assignments, err := s.repo.ListCounterReconciliationAssignments(ctx, counter)
	if err != nil {
		return nil, err
	}
	matched := make([]ReconciliationEvent, 0, len(events))
	for _, event := range events {
		assignment, ok := effectiveAssignment(assignments, event.EventTime)
		if !ok {
			continue
		}
		from, _ := counterWindow(event.EventTime, assignment.PeriodAnchorAt, counter.Period)
		if from.Equal(counter.PeriodStart) {
			matched = append(matched, event)
		}
	}
	return matched, nil
}

func counterSourceComplete(counter CounterReconciliationRow, now, pruneCutoff time.Time) bool {
	retentionComplete := counter.EventRetentionDays > 0 && !counter.PeriodStart.Before(now.AddDate(0, 0, -counter.EventRetentionDays))
	pruneComplete := pruneCutoff.IsZero() || !counter.PeriodStart.Before(pruneCutoff)
	return retentionComplete && pruneComplete
}

func effectiveAssignment(assignments []ReconciliationAssignment, at time.Time) (ReconciliationAssignment, bool) {
	for _, assignment := range assignments {
		if !assignment.AssignedAt.After(at) && (assignment.UnassignedAt.IsZero() || assignment.UnassignedAt.After(at)) {
			return assignment, true
		}
	}
	return ReconciliationAssignment{}, false
}

func counterWindow(at, anchor time.Time, period string) (time.Time, time.Time) {
	from, to := anchor.UTC(), addCounterPeriod(anchor.UTC(), period)
	for !at.UTC().Before(to) {
		from, to = to, addCounterPeriod(to, period)
	}
	return from, to
}

func addCounterPeriod(from time.Time, period string) time.Time {
	switch period {
	case "day":
		return from.AddDate(0, 0, 1)
	case "week":
		return from.AddDate(0, 0, 7)
	case "year":
		return from.AddDate(1, 0, 0)
	default:
		return from.AddDate(0, 1, 0)
	}
}

func counterAggregates(events []ReconciliationEvent) (sum, min, max float64) {
	if len(events) == 0 {
		return 0, 0, 0
	}
	min, max = events[0].Quantity, events[0].Quantity
	for _, event := range events {
		sum += event.Quantity
		min = math.Min(min, event.Quantity)
		max = math.Max(max, event.Quantity)
	}
	return sum, min, max
}

func counterValueIssue(base ReconciliationIssue, kind string, expected, actual float64, field string) ReconciliationIssue {
	base.Kind, base.Expected, base.Actual = kind, fmt.Sprintf("%.10g", expected), fmt.Sprintf("%.10g", actual)
	base.Message = "The active quota counter " + field + " differs from source usage."
	return base
}

func floatEqual(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}
