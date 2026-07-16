package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
)

func (r *SystemRepository) ListRollupMeterCoverage(ctx context.Context) ([]appsystem.RollupMeterCoverage, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.store.QueryContext(ctx, `
		WITH latest AS (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY workspace_id, meter_name ORDER BY finalized_through DESC, id DESC) AS rank
			FROM usage_rollup_runs WHERE workspace_id = $1
		)
		SELECT m.name, m.event_retention_days, r.finalized_through, r.source_events,
			r.rollup_rows, r.created_at,
			(SELECT COUNT(*) FROM usage_hourly_rollups h
			 WHERE h.workspace_id = m.workspace_id AND h.meter_name = m.name AND (
				h.event_count <= 0 OR h.quantity_min > h.quantity_max OR
				date_trunc('hour', h.bucket_start::timestamptz) <> h.bucket_start::timestamptz OR
				h.first_event_time::timestamptz < h.bucket_start::timestamptz OR
				h.first_event_time::timestamptz >= h.bucket_start::timestamptz + interval '1 hour' OR
				h.last_event_time::timestamptz < h.bucket_start::timestamptz OR
				h.last_event_time::timestamptz >= h.bucket_start::timestamptz + interval '1 hour'
			 )),
			(SELECT COUNT(*) FROM usage_events e
			 WHERE e.workspace_id = m.workspace_id AND e.meter_name = m.name
			   AND r.finalized_through IS NOT NULL AND e.event_time < r.finalized_through)
		FROM meters m
		LEFT JOIN latest r ON r.workspace_id = m.workspace_id AND r.meter_name = m.name AND r.rank = 1
		WHERE m.workspace_id = $1 AND m.event_retention_days > 0
		ORDER BY m.name`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []appsystem.RollupMeterCoverage{}
	for rows.Next() {
		var item appsystem.RollupMeterCoverage
		var finalized, created sql.NullString
		var source, rollup sql.NullInt64
		if err := rows.Scan(&item.MeterName, &item.RetentionDays, &finalized, &source, &rollup, &created, &item.InvalidRollupRows, &item.RawBeforeFinalized); err != nil {
			return nil, err
		}
		item.SourceEvents, item.RollupRows = source.Int64, rollup.Int64
		if finalized.Valid {
			item.FinalizedThrough, err = time.Parse(time.RFC3339Nano, finalized.String)
		}
		if created.Valid {
			var parseErr error
			item.LastRunAt, parseErr = time.Parse(time.RFC3339Nano, created.String)
			err = errors.Join(err, parseErr)
		}
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
