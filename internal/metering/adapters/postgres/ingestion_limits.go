package postgres

import (
	"context"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
)

func (r *UsageRepository) ConsumeIngestionCapacity(ctx context.Context, windowStart time.Time, requested, limit int, updatedAt time.Time) (bool, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return false, err
	}
	allowed := false
	err = r.store.WithinTransaction(ctx, func(txCtx context.Context) error {
		if _, err := r.store.ExecContext(txCtx, `INSERT INTO ingestion_rate_windows (workspace_id, window_start, permitted_events, throttled_events, updated_at)
			VALUES ($1, $2, 0, 0, $3) ON CONFLICT (workspace_id) DO NOTHING`, workspaceID, formatTime(windowStart), formatTime(updatedAt)); err != nil {
			return err
		}
		result, err := r.store.ExecContext(txCtx, `UPDATE ingestion_rate_windows SET
			window_start = $2,
			permitted_events = CASE WHEN window_start = $2 THEN permitted_events ELSE 0 END + $3,
			throttled_events = CASE WHEN window_start = $2 THEN throttled_events ELSE 0 END,
			updated_at = $5
			WHERE workspace_id = $1 AND (CASE WHEN window_start = $2 THEN permitted_events ELSE 0 END) + $3 <= $4`,
			workspaceID, formatTime(windowStart), requested, limit, formatTime(updatedAt))
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		allowed = rows == 1
		if allowed {
			return nil
		}
		if _, err := r.store.ExecContext(txCtx, `UPDATE ingestion_rate_windows SET
			window_start = $2, permitted_events = CASE WHEN window_start = $2 THEN permitted_events ELSE 0 END,
			throttled_events = CASE WHEN window_start = $2 THEN throttled_events ELSE 0 END + $3, updated_at = $4
			WHERE workspace_id = $1`, workspaceID, formatTime(windowStart), requested, formatTime(updatedAt)); err != nil {
			return err
		}
		_, err = r.store.ExecContext(txCtx, `INSERT INTO workspace_stats
			(workspace_id, meters, usage_events, prune_runs, ingestion_throttled, updated_at)
			VALUES ($1, 0, 0, 0, $2, $3)
			ON CONFLICT (workspace_id) DO UPDATE SET
			ingestion_throttled = workspace_stats.ingestion_throttled + EXCLUDED.ingestion_throttled,
			updated_at = EXCLUDED.updated_at`, workspaceID, requested, formatTime(updatedAt))
		return err
	})
	return allowed, err
}
