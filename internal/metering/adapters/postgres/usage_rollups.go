package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

func (r *UsageRepository) validateRollupQuery(ctx context.Context, meter string, from, to time.Time, filter domainusage.Filter) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	var overlaps bool
	fromText, toText := "0001-01-01T00:00:00Z", "9999-12-31T23:59:59Z"
	if !from.IsZero() {
		fromText = formatTime(from.UTC().Truncate(time.Hour))
	}
	if !to.IsZero() {
		toText = formatTime(to)
	}
	err = r.store.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM usage_hourly_rollups
		WHERE workspace_id = $1 AND meter_name = $2
			AND bucket_start >= $3 AND bucket_start < $4
	)`, workspaceID, meter, fromText, toText).Scan(&overlaps)
	if err != nil || !overlaps {
		return err
	}
	if (!from.IsZero() && !from.Equal(from.UTC().Truncate(time.Hour))) || (!to.IsZero() && !to.Equal(to.UTC().Truncate(time.Hour))) {
		return errors.Join(domain.ErrConflict, fmt.Errorf("retained usage queries must use UTC hour-aligned from and to timestamps"))
	}
	if !filter.RollupCompatible() {
		return errors.Join(domain.ErrConflict, fmt.Errorf("retained usage cannot be filtered by event-only fields"))
	}
	return nil
}

func (r *UsageRepository) rollUpPrunableEvents(ctx context.Context, workspaceID string, query domainusage.PruneQuery) (int, error) {
	var sourceCount int
	if err := r.store.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_events
		WHERE workspace_id = $1 AND meter_name = $2 AND event_time < $3`,
		workspaceID, query.MeterName(), formatTime(query.Before())).Scan(&sourceCount); err != nil {
		return 0, err
	}
	if sourceCount == 0 {
		if err := r.saveRollupRun(ctx, workspaceID, query, 0, 0); err != nil {
			return 0, err
		}
		return 0, nil
	}
	result, err := r.store.ExecContext(ctx, `
		INSERT INTO usage_hourly_rollups (
			workspace_id, meter_name, subject, bucket_start, metadata, event_count,
			quantity_sum, quantity_min, quantity_max,
			first_quantity, first_event_time, first_event_id,
			last_quantity, last_event_time, last_event_id, rolled_up_at
		)
		SELECT workspace_id, meter_name, subject,
			to_char(date_trunc('hour', event_time::timestamptz AT TIME ZONE 'UTC'), 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			metadata, COUNT(*), SUM(quantity), MIN(quantity), MAX(quantity),
			(array_agg(quantity ORDER BY event_time::timestamptz ASC, id ASC))[1],
			(array_agg(event_time ORDER BY event_time::timestamptz ASC, id ASC))[1],
			(array_agg(id ORDER BY event_time::timestamptz ASC, id ASC))[1],
			(array_agg(quantity ORDER BY event_time::timestamptz DESC, id DESC))[1],
			(array_agg(event_time ORDER BY event_time::timestamptz DESC, id DESC))[1],
			(array_agg(id ORDER BY event_time::timestamptz DESC, id DESC))[1], $4
		FROM usage_events
		WHERE workspace_id = $1 AND meter_name = $2 AND event_time < $3
		GROUP BY workspace_id, meter_name, subject,
			date_trunc('hour', event_time::timestamptz AT TIME ZONE 'UTC'), metadata
		ON CONFLICT (workspace_id, meter_name, subject, bucket_start, metadata) DO UPDATE SET
			event_count = usage_hourly_rollups.event_count + EXCLUDED.event_count,
			quantity_sum = usage_hourly_rollups.quantity_sum + EXCLUDED.quantity_sum,
			quantity_min = LEAST(usage_hourly_rollups.quantity_min, EXCLUDED.quantity_min),
			quantity_max = GREATEST(usage_hourly_rollups.quantity_max, EXCLUDED.quantity_max),
			first_quantity = CASE WHEN (EXCLUDED.first_event_time::timestamptz, EXCLUDED.first_event_id) < (usage_hourly_rollups.first_event_time::timestamptz, usage_hourly_rollups.first_event_id) THEN EXCLUDED.first_quantity ELSE usage_hourly_rollups.first_quantity END,
			first_event_time = CASE WHEN EXCLUDED.first_event_time::timestamptz < usage_hourly_rollups.first_event_time::timestamptz THEN EXCLUDED.first_event_time ELSE usage_hourly_rollups.first_event_time END,
			first_event_id = CASE WHEN (EXCLUDED.first_event_time::timestamptz, EXCLUDED.first_event_id) < (usage_hourly_rollups.first_event_time::timestamptz, usage_hourly_rollups.first_event_id) THEN EXCLUDED.first_event_id ELSE usage_hourly_rollups.first_event_id END,
			last_quantity = CASE WHEN (EXCLUDED.last_event_time::timestamptz, EXCLUDED.last_event_id) > (usage_hourly_rollups.last_event_time::timestamptz, usage_hourly_rollups.last_event_id) THEN EXCLUDED.last_quantity ELSE usage_hourly_rollups.last_quantity END,
			last_event_time = CASE WHEN EXCLUDED.last_event_time::timestamptz > usage_hourly_rollups.last_event_time::timestamptz THEN EXCLUDED.last_event_time ELSE usage_hourly_rollups.last_event_time END,
			last_event_id = CASE WHEN (EXCLUDED.last_event_time::timestamptz, EXCLUDED.last_event_id) > (usage_hourly_rollups.last_event_time::timestamptz, usage_hourly_rollups.last_event_id) THEN EXCLUDED.last_event_id ELSE usage_hourly_rollups.last_event_id END,
			rolled_up_at = EXCLUDED.rolled_up_at`,
		workspaceID, query.MeterName(), formatTime(query.Before()), formatTime(time.Now().UTC()))
	if err != nil {
		return 0, err
	}
	rollupRows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := r.saveRollupRun(ctx, workspaceID, query, sourceCount, rollupRows); err != nil {
		return 0, err
	}
	return sourceCount, nil
}

func (r *UsageRepository) saveRollupRun(ctx context.Context, workspaceID string, query domainusage.PruneQuery, sourceEvents int, rollupRows int64) error {
	_, err := r.store.ExecContext(ctx, `INSERT INTO usage_rollup_runs
		(workspace_id, meter_name, finalized_through, source_events, rollup_rows, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (workspace_id, meter_name, finalized_through) DO NOTHING`,
		workspaceID, query.MeterName(), formatTime(query.Before()), sourceEvents, rollupRows, formatTime(time.Now().UTC()))
	return err
}
