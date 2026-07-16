package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
)

func (r *SystemRepository) PruneOperationalHistory(ctx context.Context, before time.Time, batchSize int) (appsystem.OperationalHistoryPruneResult, error) {
	beforeText := before.UTC().Format(time.RFC3339Nano)
	result := appsystem.OperationalHistoryPruneResult{}
	steps := []struct {
		name  string
		query string
		args  []any
		set   func(int)
	}{
		{"ingestion audits", `DELETE FROM usage_ingestions WHERE id IN (SELECT id FROM usage_ingestions WHERE created_at < $1 ORDER BY created_at, id LIMIT $2)`, []any{beforeText, batchSize}, func(n int) { result.IngestionAudits = n }},
		{"usage prune runs", `DELETE FROM usage_prune_runs WHERE id IN (SELECT id FROM usage_prune_runs WHERE created_at < $1 ORDER BY created_at, id LIMIT $2)`, []any{beforeText, batchSize}, func(n int) { result.UsagePruneRuns = n }},
		{"decision prune runs", `DELETE FROM consumption_decision_prune_runs WHERE id IN (SELECT id FROM consumption_decision_prune_runs WHERE created_at < $1 ORDER BY created_at, id LIMIT $2)`, []any{before, batchSize}, func(n int) { result.DecisionPruneRuns = n }},
		{"export cleanup runs", `DELETE FROM usage_export_cleanup_runs WHERE id IN (SELECT id FROM usage_export_cleanup_runs WHERE created_at < $1 ORDER BY created_at, id LIMIT $2)`, []any{beforeText, batchSize}, func(n int) { result.ExportCleanupRuns = n }},
		{"expired export jobs", `DELETE FROM usage_export_jobs WHERE id IN (SELECT id FROM usage_export_jobs WHERE status = 'completed' AND expired_at IS NOT NULL AND completed_at < $1 ORDER BY completed_at, id LIMIT $2)`, []any{beforeText, batchSize}, func(n int) { result.ExportJobs = n }},
		{"delivered alert attempts", `DELETE FROM alert_deliveries WHERE id IN (SELECT d.id FROM alert_deliveries d JOIN alert_delivery_jobs j ON j.event_id = d.event_id WHERE j.status = 'delivered' AND j.delivered_at < $1 ORDER BY d.attempted_at, d.id LIMIT $2)`, []any{before, batchSize}, func(n int) { result.AlertDeliveries = n }},
		{"delivered alert jobs", `DELETE FROM alert_delivery_jobs WHERE id IN (SELECT j.id FROM alert_delivery_jobs j WHERE j.status = 'delivered' AND j.delivered_at < $1 AND NOT EXISTS (SELECT 1 FROM alert_deliveries d WHERE d.event_id = j.event_id) ORDER BY j.delivered_at, j.id LIMIT $2)`, []any{before, batchSize}, func(n int) { result.AlertDeliveryJobs = n }},
		{"reconciliation runs", `DELETE FROM reconciliation_runs WHERE id IN (SELECT id FROM reconciliation_runs WHERE created_at < $1 ORDER BY created_at, id LIMIT $2)`, []any{before, batchSize}, func(n int) { result.ReconciliationRuns = n }},
		{"delivered reconciliation notifications", `DELETE FROM reconciliation_notifications WHERE id IN (SELECT id FROM reconciliation_notifications WHERE status = 'delivered' AND delivered_at < $1 ORDER BY delivered_at, id LIMIT $2)`, []any{before, batchSize}, func(n int) { result.ReconciliationNotifications = n }},
		{"superseded rollup runs", `DELETE FROM usage_rollup_runs r WHERE r.id IN (SELECT old.id FROM usage_rollup_runs old WHERE old.created_at < $1 AND EXISTS (SELECT 1 FROM usage_rollup_runs newer WHERE newer.workspace_id = old.workspace_id AND newer.meter_name = old.meter_name AND newer.id > old.id) ORDER BY old.created_at, old.id LIMIT $2)`, []any{beforeText, batchSize}, func(n int) { result.RollupRuns = n }},
	}
	for _, step := range steps {
		count, err := execHistoryPrune(ctx, r.store, step.query, step.args...)
		if err != nil {
			return appsystem.OperationalHistoryPruneResult{}, fmt.Errorf("prune %s: %w", step.name, err)
		}
		step.set(count)
	}
	return result, nil
}

type historyExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func execHistoryPrune(ctx context.Context, execer historyExecer, query string, args ...any) (int, error) {
	result, err := execer.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	return int(rows), err
}
