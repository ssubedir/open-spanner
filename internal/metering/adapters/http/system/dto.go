package system

// StatsResponse is operational system stats.
type StatsResponse struct {
	Meters                int                           `json:"meters"`
	UsageEvents           int                           `json:"usage_events"`
	PruneRuns             int                           `json:"prune_runs"`
	LastPruneRun          *LastPruneRunResponse         `json:"last_prune_run"`
	ConsumptionDecisions  int                           `json:"consumption_decisions"`
	DecisionPruneRuns     int                           `json:"decision_prune_runs"`
	LastDecisionPruneRun  *LastDecisionPruneRunResponse `json:"last_decision_prune_run"`
	LastReconciliationRun *ReconciliationRunResponse    `json:"last_reconciliation_run"`
	ReconciliationHealth  ReconciliationHealthResponse  `json:"reconciliation_health"`
	WorkerHealth          []WorkerHealthResponse        `json:"worker_health"`
}

type WorkerHealthResponse struct {
	Name            string `json:"name"`
	Status          string `json:"status"`
	StartedAt       string `json:"started_at,omitempty"`
	LastHeartbeatAt string `json:"last_heartbeat_at,omitempty"`
	PendingJobs     int    `json:"pending_jobs"`
	RunningJobs     int    `json:"running_jobs"`
	FailedJobs      int    `json:"failed_jobs"`
	OldestPendingAt string `json:"oldest_pending_at,omitempty"`
	LastSuccessAt   string `json:"last_success_at,omitempty"`
	LastFailureAt   string `json:"last_failure_at,omitempty"`
}

type LastDecisionPruneRunResponse struct {
	ID        string `json:"id"`
	Before    string `json:"before"`
	Deleted   int    `json:"deleted"`
	DryRun    bool   `json:"dry_run"`
	CreatedAt string `json:"created_at"`
}

// LastPruneRunResponse is the most recent prune run summary.
type LastPruneRunResponse struct {
	ID        string `json:"id"`
	Deleted   int    `json:"deleted"`
	DryRun    bool   `json:"dry_run"`
	CreatedAt string `json:"created_at"`
}

// ReconciliationResponse reports read-only quota consistency checks.
type ReconciliationResponse struct {
	Status           string                        `json:"status"`
	DecisionsChecked int                           `json:"decisions_checked"`
	CountersChecked  int                           `json:"counters_checked"`
	Issues           []ReconciliationIssueResponse `json:"issues"`
	Truncated        bool                          `json:"truncated"`
	LookbackHours    int                           `json:"lookback_hours"`
	CheckedAt        string                        `json:"checked_at"`
}

// ReconciliationIssueResponse describes one detected inconsistency.
type ReconciliationIssueResponse struct {
	Kind           string `json:"kind"`
	Severity       string `json:"severity"`
	Subject        string `json:"subject,omitempty"`
	Meter          string `json:"meter,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	Period         string `json:"period,omitempty"`
	PeriodStart    string `json:"period_start,omitempty"`
	Expected       string `json:"expected"`
	Actual         string `json:"actual"`
	Message        string `json:"message"`
}

type ReconciliationRunResponse struct {
	ID               string                        `json:"id"`
	Status           string                        `json:"status"`
	DecisionsChecked int                           `json:"decisions_checked"`
	CountersChecked  int                           `json:"counters_checked"`
	IssueCount       int                           `json:"issue_count"`
	Truncated        bool                          `json:"truncated"`
	LookbackHours    int                           `json:"lookback_hours"`
	DurationMS       int64                         `json:"duration_ms"`
	Fingerprint      string                        `json:"fingerprint,omitempty"`
	Issues           []ReconciliationIssueResponse `json:"issues"`
	Error            string                        `json:"error,omitempty"`
	CreatedAt        string                        `json:"created_at"`
}

type ReconciliationRunListResponse struct {
	Items []ReconciliationRunResponse `json:"items"`
}

type ReconciliationHealthResponse struct {
	Status                  string `json:"status"`
	NextRunAt               string `json:"next_run_at,omitempty"`
	LockedUntil             string `json:"locked_until,omitempty"`
	UpdatedAt               string `json:"updated_at,omitempty"`
	PendingNotifications    int    `json:"pending_notifications"`
	DeadLetterNotifications int    `json:"dead_letter_notifications"`
}

type ReconciliationNotificationResponse struct {
	ID             string                                      `json:"id"`
	EventType      string                                      `json:"event_type"`
	Status         string                                      `json:"status"`
	Attempts       int                                         `json:"attempts"`
	TotalAttempts  int                                         `json:"total_attempts"`
	NextAttemptAt  string                                      `json:"next_attempt_at"`
	LastError      string                                      `json:"last_error,omitempty"`
	CreatedAt      string                                      `json:"created_at"`
	DeliveredAt    string                                      `json:"delivered_at,omitempty"`
	AttemptHistory []ReconciliationNotificationAttemptResponse `json:"attempt_history"`
}

type ReconciliationNotificationAttemptResponse struct {
	Attempt   int    `json:"attempt"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"created_at"`
}

type ReconciliationNotificationListResponse struct {
	Items []ReconciliationNotificationResponse `json:"items"`
}

// CounterRepairRequest previews or applies one targeted quota counter repair.
type CounterRepairRequest struct {
	Subject           string `json:"subject"`
	Meter             string `json:"meter"`
	Period            string `json:"period"`
	PeriodStart       string `json:"period_start"`
	DryRun            *bool  `json:"dry_run"`
	ExpectedUpdatedAt string `json:"expected_updated_at,omitempty"`
}

// CounterSnapshotResponse is the derivable state of one quota counter.
type CounterSnapshotResponse struct {
	EventCount     int64   `json:"event_count"`
	QuantitySum    float64 `json:"quantity_sum"`
	QuantityMin    float64 `json:"quantity_min"`
	QuantityMax    float64 `json:"quantity_max"`
	FirstQuantity  float64 `json:"first_quantity"`
	FirstEventTime string  `json:"first_event_time,omitempty"`
	LastQuantity   float64 `json:"last_quantity"`
	LastEventTime  string  `json:"last_event_time,omitempty"`
}

// CounterRepairResponse is an audited repair preview or application.
type CounterRepairResponse struct {
	ID               string                  `json:"id"`
	Subject          string                  `json:"subject"`
	Meter            string                  `json:"meter"`
	Period           string                  `json:"period"`
	PeriodStart      string                  `json:"period_start"`
	PeriodEnd        string                  `json:"period_end"`
	DryRun           bool                    `json:"dry_run"`
	Applied          bool                    `json:"applied"`
	Before           CounterSnapshotResponse `json:"before"`
	After            CounterSnapshotResponse `json:"after"`
	CounterUpdatedAt string                  `json:"counter_updated_at"`
	CreatedAt        string                  `json:"created_at"`
}

type CounterRepairListResponse struct {
	Items []CounterRepairResponse `json:"items"`
}
