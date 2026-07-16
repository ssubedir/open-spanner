package sqlite

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
	var overlaps int
	fromText, toText := "0001-01-01T00:00:00Z", "9999-12-31T23:59:59Z"
	if !from.IsZero() {
		fromText = formatTime(from.UTC().Truncate(time.Hour))
	}
	if !to.IsZero() {
		toText = formatTime(to)
	}
	err = r.store.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM usage_hourly_rollups
		WHERE workspace_id = ? AND meter_name = ?
			AND bucket_start >= ? AND bucket_start < ?
	)`, workspaceID, meter, fromText, toText).Scan(&overlaps)
	if err != nil || overlaps == 0 {
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
		WHERE workspace_id = ? AND meter_name = ? AND event_time < ?`,
		workspaceID, query.MeterName(), formatTime(query.Before())).Scan(&sourceCount); err != nil {
		return 0, err
	}
	if sourceCount == 0 {
		return 0, nil
	}
	_, err := r.store.ExecContext(ctx, `
		WITH ranked AS (
			SELECT *, strftime('%Y-%m-%dT%H:00:00Z', event_time) AS bucket,
				ROW_NUMBER() OVER (PARTITION BY subject, strftime('%Y-%m-%dT%H:00:00Z', event_time), metadata ORDER BY julianday(event_time), id) AS first_rank,
				ROW_NUMBER() OVER (PARTITION BY subject, strftime('%Y-%m-%dT%H:00:00Z', event_time), metadata ORDER BY julianday(event_time) DESC, id DESC) AS last_rank
			FROM usage_events
			WHERE workspace_id = ? AND meter_name = ? AND event_time < ?
		)
		INSERT INTO usage_hourly_rollups (
			workspace_id, meter_name, subject, bucket_start, metadata, event_count,
			quantity_sum, quantity_min, quantity_max,
			first_quantity, first_event_time, first_event_id,
			last_quantity, last_event_time, last_event_id, rolled_up_at
		)
		SELECT workspace_id, meter_name, subject, bucket, metadata, COUNT(*),
			SUM(quantity), MIN(quantity), MAX(quantity),
			MAX(CASE WHEN first_rank = 1 THEN quantity END),
			MAX(CASE WHEN first_rank = 1 THEN event_time END),
			MAX(CASE WHEN first_rank = 1 THEN id END),
			MAX(CASE WHEN last_rank = 1 THEN quantity END),
			MAX(CASE WHEN last_rank = 1 THEN event_time END),
			MAX(CASE WHEN last_rank = 1 THEN id END), ?
		FROM ranked GROUP BY workspace_id, meter_name, subject, bucket, metadata
		ON CONFLICT (workspace_id, meter_name, subject, bucket_start, metadata) DO UPDATE SET
			event_count = event_count + excluded.event_count,
			quantity_sum = quantity_sum + excluded.quantity_sum,
			quantity_min = MIN(quantity_min, excluded.quantity_min),
			quantity_max = MAX(quantity_max, excluded.quantity_max),
			first_quantity = CASE WHEN julianday(excluded.first_event_time) < julianday(first_event_time) OR (julianday(excluded.first_event_time) = julianday(first_event_time) AND excluded.first_event_id < first_event_id) THEN excluded.first_quantity ELSE first_quantity END,
			first_event_time = CASE WHEN julianday(excluded.first_event_time) < julianday(first_event_time) THEN excluded.first_event_time ELSE first_event_time END,
			first_event_id = CASE WHEN julianday(excluded.first_event_time) < julianday(first_event_time) OR (julianday(excluded.first_event_time) = julianday(first_event_time) AND excluded.first_event_id < first_event_id) THEN excluded.first_event_id ELSE first_event_id END,
			last_quantity = CASE WHEN julianday(excluded.last_event_time) > julianday(last_event_time) OR (julianday(excluded.last_event_time) = julianday(last_event_time) AND excluded.last_event_id > last_event_id) THEN excluded.last_quantity ELSE last_quantity END,
			last_event_time = CASE WHEN julianday(excluded.last_event_time) > julianday(last_event_time) THEN excluded.last_event_time ELSE last_event_time END,
			last_event_id = CASE WHEN julianday(excluded.last_event_time) > julianday(last_event_time) OR (julianday(excluded.last_event_time) = julianday(last_event_time) AND excluded.last_event_id > last_event_id) THEN excluded.last_event_id ELSE last_event_id END,
			rolled_up_at = excluded.rolled_up_at`,
		workspaceID, query.MeterName(), formatTime(query.Before()), formatTime(time.Now().UTC()))
	return sourceCount, err
}
