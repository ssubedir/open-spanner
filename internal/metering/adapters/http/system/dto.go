package system

// StatsResponse is operational system stats.
type StatsResponse struct {
	Meters               int                           `json:"meters"`
	UsageEvents          int                           `json:"usage_events"`
	PruneRuns            int                           `json:"prune_runs"`
	LastPruneRun         *LastPruneRunResponse         `json:"last_prune_run"`
	ConsumptionDecisions int                           `json:"consumption_decisions"`
	DecisionPruneRuns    int                           `json:"decision_prune_runs"`
	LastDecisionPruneRun *LastDecisionPruneRunResponse `json:"last_decision_prune_run"`
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
