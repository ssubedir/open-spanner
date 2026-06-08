package system

// StatsResponse is operational system stats.
type StatsResponse struct {
	Meters       int                   `json:"meters"`
	UsageEvents  int                   `json:"usage_events"`
	PruneRuns    int                   `json:"prune_runs"`
	LastPruneRun *LastPruneRunResponse `json:"last_prune_run"`
}

// LastPruneRunResponse is the most recent prune run summary.
type LastPruneRunResponse struct {
	ID        string `json:"id"`
	Deleted   int    `json:"deleted"`
	DryRun    bool   `json:"dry_run"`
	CreatedAt string `json:"created_at"`
}
