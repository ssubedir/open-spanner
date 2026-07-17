package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/internal/usagebatch"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/postgres/postgresdb"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

var metadataKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)

var errBulkReplay = errors.New("bulk ingestion already exists")

const (
	pruneAdvisoryLockKey = int64(0x4f535052554e45)
	pruneDeleteBatchSize = 1000
	usageInsertBatchSize = 100
)

type UsageRepository struct {
	store   *Store
	queries *postgresdb.Queries
}

type eventSnapshot struct {
	ID             string         `json:"id"`
	IdempotencyKey string         `json:"idempotency_key"`
	Subject        string         `json:"subject"`
	MeterName      string         `json:"meter_name"`
	Quantity       float64        `json:"quantity"`
	EventTime      string         `json:"event_time"`
	ReceivedAt     string         `json:"received_at"`
	Metadata       map[string]any `json:"metadata"`
}

type bulkSnapshot struct {
	Accepted   []eventSnapshot `json:"accepted"`
	Duplicates []eventSnapshot `json:"duplicates"`
}

type pruneRunMeterSnapshot struct {
	MeterName string `json:"meter_name"`
	Before    string `json:"before"`
	Deleted   int    `json:"deleted"`
}

func NewUsageRepository(store *Store) *UsageRepository {
	return &UsageRepository{store: store, queries: postgresdb.New(store)}
}

func (r *UsageRepository) EnqueueOutbox(ctx context.Context, events []domainusage.Event, now time.Time) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	for _, event := range events {
		metadata, err := json.Marshal(event.Metadata())
		if err != nil {
			return err
		}
		if err := queriesFor(ctx, r.queries).EnqueueUsageEventOutbox(ctx, postgresdb.EnqueueUsageEventOutboxParams{
			PublicID: uuid.Must(uuid.NewV7()), WorkspaceID: workspaceID, EventID: event.ID(), Subject: event.Subject(),
			MeterName: event.MeterName(), Quantity: event.Quantity(), Metadata: metadata, Now: now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *UsageRepository) ClaimOutbox(ctx context.Context, now, lockedUntil time.Time, claimToken string, maxAttempts int) (domainusage.OutboxMessage, error) {
	token, err := uuid.Parse(claimToken)
	if err != nil {
		return domainusage.OutboxMessage{}, err
	}
	row, err := queriesFor(ctx, r.queries).ClaimUsageEventOutbox(ctx, postgresdb.ClaimUsageEventOutboxParams{Now: now, LockedUntil: lockedUntil, ClaimToken: token, MaxAttempts: int32(maxAttempts)})
	if errors.Is(err, sql.ErrNoRows) {
		return domainusage.OutboxMessage{}, domain.ErrNotFound
	}
	if err != nil {
		return domainusage.OutboxMessage{}, err
	}
	metadata := map[string]any{}
	if err := json.Unmarshal(row.Metadata, &metadata); err != nil {
		return domainusage.OutboxMessage{}, err
	}
	return domainusage.OutboxMessage{ID: row.PublicID.String(), WorkspaceID: row.WorkspaceID, EventID: row.EventID, Subject: row.Subject, MeterName: row.MeterName, Quantity: row.Quantity, Metadata: metadata, Attempts: int(row.Attempts), ClaimToken: row.ClaimToken.UUID.String(), CreatedAt: row.CreatedAt}, nil
}

func (r *UsageRepository) CompleteOutbox(ctx context.Context, id, claimToken string, _ time.Time) error {
	publicID, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	token, err := uuid.Parse(claimToken)
	if err != nil {
		return err
	}
	rows, err := queriesFor(ctx, r.queries).CompleteUsageEventOutbox(ctx, postgresdb.CompleteUsageEventOutboxParams{PublicID: publicID, ClaimToken: token})
	if err == nil && rows == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (r *UsageRepository) RetryOutbox(ctx context.Context, id, claimToken string, nextAttemptAt time.Time, maxAttempts int, lastError string, now time.Time) error {
	publicID, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	token, err := uuid.Parse(claimToken)
	if err != nil {
		return err
	}
	rows, err := queriesFor(ctx, r.queries).RetryUsageEventOutbox(ctx, postgresdb.RetryUsageEventOutboxParams{PublicID: publicID, ClaimToken: token, NextAttemptAt: nextAttemptAt, MaxAttempts: int32(maxAttempts), LastError: lastError, Now: now})
	if err == nil && rows == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (r *UsageRepository) Save(ctx context.Context, event domainusage.Event) (domainusage.Event, error) {
	return r.save(ctx, event)
}

func (r *UsageRepository) FindEventByIdempotencyKey(ctx context.Context, idempotencyKey string) (domainusage.Event, error) {
	event, err := r.findByIdempotencyKey(ctx, idempotencyKey)
	if errors.Is(err, sql.ErrNoRows) {
		return domainusage.Event{}, domain.ErrNotFound
	}
	return event, err
}

func (r *UsageRepository) SaveBulk(ctx context.Context, idempotencyKey string, events []domainusage.Event) (domainusage.BulkSaveResult, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.BulkSaveResult{}, err
	}
	if idempotencyKey != "" {
		existing, err := r.findBulk(ctx, idempotencyKey)
		if err == nil {
			return existing.AsReplay(), nil
		}
		if err != sql.ErrNoRows {
			return domainusage.BulkSaveResult{}, err
		}
	}

	var result domainusage.BulkSaveResult
	err = r.store.WithinTransaction(ctx, func(txCtx context.Context) error {
		var err error
		result, err = r.saveEventBatch(txCtx, workspaceID, events)
		if err != nil {
			return err
		}
		if idempotencyKey == "" {
			return nil
		}

		response, err := marshalBulkResult(result)
		if err != nil {
			return err
		}

		rowsAffected, err := queriesFor(txCtx, r.queries).SaveBulkUsageIngestion(txCtx, postgresdb.SaveBulkUsageIngestionParams{
			IdempotencyKey: idempotencyKey,
			WorkspaceID:    workspaceID,
			Response:       response,
			CreatedAt:      formatTime(time.Now().UTC()),
		})
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return errBulkReplay
		}

		return nil
	})
	if errors.Is(err, errBulkReplay) {
		existing, findErr := r.findBulk(ctx, idempotencyKey)
		return existing.AsReplay(), findErr
	}
	if err != nil {
		return domainusage.BulkSaveResult{}, err
	}

	return result, nil
}

func (r *UsageRepository) save(ctx context.Context, event domainusage.Event) (domainusage.Event, error) {
	var saved domainusage.Event
	err := r.store.WithinTransaction(ctx, func(txCtx context.Context) error {
		workspaceID, err := appauth.RequireWorkspaceID(txCtx)
		if err != nil {
			return err
		}
		result, err := r.saveEventBatch(txCtx, workspaceID, []domainusage.Event{event})
		if err != nil {
			return err
		}
		if len(result.Accepted()) == 1 {
			saved = result.Accepted()[0]
		} else {
			saved = result.Duplicates()[0]
		}
		return nil
	})
	return saved, err
}

func (r *UsageRepository) saveEventBatch(ctx context.Context, workspaceID string, events []domainusage.Event) (domainusage.BulkSaveResult, error) {
	inserted, err := r.insertUsageEvents(ctx, workspaceID, events)
	if err != nil {
		return domainusage.BulkSaveResult{}, err
	}
	accepted := make([]domainusage.Event, 0, len(inserted))
	duplicates := make([]domainusage.Event, 0, len(events)-len(inserted))
	for _, event := range events {
		if _, ok := inserted[event.ID()]; ok {
			accepted = append(accepted, event)
			continue
		}
		if _, findErr := r.findByID(ctx, event.ID()); findErr == nil {
			return domainusage.BulkSaveResult{}, domain.ErrConflict
		} else if findErr != sql.ErrNoRows {
			return domainusage.BulkSaveResult{}, findErr
		}
		if event.IdempotencyKey() == "" {
			return domainusage.BulkSaveResult{}, domain.ErrConflict
		}
		existing, findErr := r.findByIdempotencyKey(ctx, event.IdempotencyKey())
		if findErr != nil {
			return domainusage.BulkSaveResult{}, findErr
		}
		duplicates = append(duplicates, existing)
	}
	if err := r.applyAcceptedUsage(ctx, workspaceID, accepted); err != nil {
		return domainusage.BulkSaveResult{}, err
	}
	return domainusage.NewBulkSaveResult(accepted, duplicates), nil
}

func (r *UsageRepository) insertUsageEvents(ctx context.Context, workspaceID string, events []domainusage.Event) (map[string]struct{}, error) {
	inserted := make(map[string]struct{}, len(events))
	for start := 0; start < len(events); start += usageInsertBatchSize {
		end := start + usageInsertBatchSize
		if end > len(events) {
			end = len(events)
		}
		var query strings.Builder
		query.WriteString(`INSERT INTO usage_events (id, workspace_id, idempotency_key, subject, meter_name, quantity, event_time, received_at, metadata) VALUES `)
		args := make([]any, 0, (end-start)*9)
		for index, event := range events[start:end] {
			if index > 0 {
				query.WriteByte(',')
			}
			base := len(args) + 1
			fmt.Fprintf(&query, "($%d,$%d,NULLIF($%d::text,''),$%d,$%d,$%d,$%d,$%d,$%d::jsonb)", base, base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8)
			metadata, err := json.Marshal(event.Metadata())
			if err != nil {
				return nil, err
			}
			args = append(args, event.ID(), workspaceID, event.IdempotencyKey(), event.Subject(), event.MeterName(), event.Quantity(), formatTime(event.EventTime()), formatTime(event.ReceivedAt()), json.RawMessage(metadata))
		}
		query.WriteString(` ON CONFLICT DO NOTHING RETURNING id`)
		rows, err := r.store.QueryContext(ctx, query.String(), args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			inserted[id] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return inserted, nil
}

func (r *UsageRepository) applyAcceptedUsage(ctx context.Context, workspaceID string, events []domainusage.Event) error {
	if len(events) == 0 {
		return nil
	}
	updatedAt := formatTime(time.Now().UTC())
	assignments, err := r.findUsagePlanAssignments(ctx, workspaceID, events)
	if err != nil {
		return err
	}
	for _, counter := range usagebatch.AggregateCounters(events, assignments) {
		if err := queriesFor(ctx, r.queries).IncrementEntitlementUsageCounter(ctx, postgresdb.IncrementEntitlementUsageCounterParams{
			WorkspaceID: workspaceID, Subject: counter.Subject, MeterName: counter.MeterName, Period: counter.Period,
			PeriodStart: formatTime(counter.PeriodStart), PeriodEnd: formatTime(counter.PeriodEnd), EventCount: counter.EventCount,
			QuantitySum: counter.QuantitySum, QuantityMin: counter.QuantityMin, QuantityMax: counter.QuantityMax,
			FirstQuantity: counter.FirstQuantity, FirstEventTime: formatTime(counter.FirstEventAt),
			LastQuantity: counter.LastQuantity, LastEventTime: formatTime(counter.LastEventAt), UpdatedAt: updatedAt,
		}); err != nil {
			return err
		}
	}
	return queriesFor(ctx, r.queries).IncrementWorkspaceUsageEvents(ctx, postgresdb.IncrementWorkspaceUsageEventsParams{
		WorkspaceID: workspaceID, Delta: int64(len(events)), UpdatedAt: updatedAt,
	})
}

func (r *UsageRepository) findUsagePlanAssignments(ctx context.Context, workspaceID string, events []domainusage.Event) (map[string][]usagebatch.Assignment, error) {
	bounds := usagebatch.SubjectBounds(events)
	subjects := make([]string, 0, len(bounds))
	for subject := range bounds {
		subjects = append(subjects, subject)
	}
	sort.Strings(subjects)
	result := make(map[string][]usagebatch.Assignment, len(subjects))
	for _, subject := range subjects {
		bound := bounds[subject]
		rows, err := r.store.QueryContext(ctx, `SELECT assigned_at, period_anchor_at, unassigned_at
			FROM plan_subject_assignments
			WHERE workspace_id = $1 AND subject = $2 AND assigned_at <= $3
				AND (unassigned_at IS NULL OR unassigned_at > $4)
			ORDER BY assigned_at DESC`, workspaceID, subject, formatTime(bound.To), formatTime(bound.From))
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var assignedAtText, anchorText string
			var unassignedAtText sql.NullString
			if err := rows.Scan(&assignedAtText, &anchorText, &unassignedAtText); err != nil {
				rows.Close()
				return nil, err
			}
			assignedAt, err := time.Parse(time.RFC3339Nano, assignedAtText)
			if err != nil {
				rows.Close()
				return nil, err
			}
			anchor, err := time.Parse(time.RFC3339Nano, anchorText)
			if err != nil {
				rows.Close()
				return nil, err
			}
			unassignedAt, err := parseOptionalTime(unassignedAtText)
			if err != nil {
				rows.Close()
				return nil, err
			}
			result[subject] = append(result[subject], usagebatch.Assignment{AssignedAt: assignedAt, Anchor: anchor, UnassignedAt: unassignedAt})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return result, nil
}

func (r *UsageRepository) Query(ctx context.Context, query domainusage.Query) ([]domainusage.Bucket, error) {
	if err := r.validateRollupQuery(ctx, query.MeterName(), query.From(), query.To(), query.Filter()); err != nil {
		return nil, err
	}
	return r.queryBucketsWithDynamicSQL(ctx, query)
}

func (r *UsageRepository) Aggregate(ctx context.Context, query domainusage.AggregateQuery) (domainusage.Aggregate, error) {
	if err := r.validateRollupQuery(ctx, query.MeterName(), query.From(), query.To(), query.Filter()); err != nil {
		return domainusage.Aggregate{}, err
	}
	return r.aggregateWithDynamicSQL(ctx, query)
}

func bucketQueryNeedsDynamicSQL(query domainusage.Query) bool {
	return !query.Filter().IsZero() || len(query.Metadata()) > 0 || len(query.GroupByFields()) > 0
}

func (r *UsageRepository) queryBucketsWithGeneratedSQL(ctx context.Context, query domainusage.Query) ([]domainusage.Bucket, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListUsageBuckets(ctx, postgresdb.ListUsageBucketsParams{
		WorkspaceID: workspaceID,
		Aggregation: string(query.Aggregation()),
		BucketSize:  string(query.BucketSize()),
		Limit:       int32(query.Limit()),
		MeterName:   query.MeterName(),
		FromTime:    formatTime(query.From()),
		ToTime:      formatTime(query.To()),
		Subject:     eventStringValue(query.Subject()),
	})
	if err != nil {
		return nil, err
	}

	buckets := make([]domainusage.Bucket, 0, len(rows))
	for _, row := range rows {
		buckets = append(buckets, domainusage.NewBucket(
			query.Subject(),
			query.MeterName(),
			query.BucketSize(),
			row.BucketStart,
			row.Quantity,
		))
	}

	return buckets, nil
}

func (r *UsageRepository) FindDimensionValues(ctx context.Context, query domainusage.DimensionValueQuery) ([]domainusage.DimensionValue, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	if !metadataKeyPattern.MatchString(query.Field()) {
		return nil, fmt.Errorf("%w: unsupported metadata field %q", domain.ErrInvalidInput, query.Field())
	}
	if err := r.validateRollupQuery(ctx, query.MeterName(), query.From(), query.To(), domainusage.EmptyFilter()); err != nil {
		return nil, err
	}
	args := []any{workspaceID, strings.Split(query.Field(), "."), query.MeterName()}
	where := "workspace_id = $1 AND meter_name = $3"
	if query.Subject() != "" {
		args = append(args, query.Subject())
		where += fmt.Sprintf(" AND subject = $%d", len(args))
	}
	if !query.From().IsZero() {
		args = append(args, formatTime(query.From()))
		where += fmt.Sprintf(" AND event_time >= $%d", len(args))
	}
	if !query.To().IsZero() {
		args = append(args, formatTime(query.To()))
		where += fmt.Sprintf(" AND event_time < $%d", len(args))
	}
	args = append(args, query.Limit())
	rows, err := r.store.QueryContext(ctx, `SELECT metadata #>> $2::text[] AS value, SUM(event_count)::bigint
		FROM usage_aggregation_fragments WHERE `+where+`
		GROUP BY metadata #>> $2::text[]
		HAVING metadata #>> $2::text[] IS NOT NULL AND metadata #>> $2::text[] <> ''
		ORDER BY SUM(event_count) DESC, metadata #>> $2::text[] ASC LIMIT $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	values := []domainusage.DimensionValue{}
	for rows.Next() {
		var value string
		var count int
		if err := rows.Scan(&value, &count); err != nil {
			return nil, err
		}
		values = append(values, domainusage.NewDimensionValue(query.Field(), value, count))
	}
	return values, rows.Err()
}

func (r *UsageRepository) FindBreakdown(ctx context.Context, query domainusage.BreakdownQuery) ([]domainusage.BreakdownItem, error) {
	if err := r.validateRollupQuery(ctx, query.MeterName(), query.From(), query.To(), query.Filter()); err != nil {
		return nil, err
	}
	return r.findBreakdownWithDynamicSQL(ctx, query)
}

func (r *UsageRepository) findBreakdownWithGeneratedSQL(ctx context.Context, query domainusage.BreakdownQuery) ([]domainusage.BreakdownItem, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := breakdownFieldExpression(query.Field()); err != nil {
		return nil, err
	}

	rows, err := queriesFor(ctx, r.queries).ListUsageBreakdown(ctx, postgresdb.ListUsageBreakdownParams{
		WorkspaceID:     workspaceID,
		Aggregation:     string(query.Aggregation()),
		DurationSeconds: query.To().Sub(query.From()).Seconds(),
		Limit:           int32(query.Limit()),
		Field:           query.Field(),
		MeterName:       query.MeterName(),
		FromTime:        formatTime(query.From()),
		ToTime:          formatTime(query.To()),
		Subject:         eventStringValue(query.Subject()),
	})
	if err != nil {
		return nil, err
	}

	items := make([]domainusage.BreakdownItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, domainusage.NewBreakdownItem(query.Field(), row.Value, row.Quantity, int(row.UsageEvents)))
	}

	return items, nil
}

func (r *UsageRepository) FindEvents(ctx context.Context, query domainusage.EventQuery) (domainusage.EventPage, error) {
	if query.Filter().IsZero() {
		return r.findEventsWithGeneratedSQL(ctx, query)
	}

	return r.findEventsWithDynamicSQL(ctx, query)
}

func (r *UsageRepository) findEventsWithGeneratedSQL(ctx context.Context, query domainusage.EventQuery) (domainusage.EventPage, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.EventPage{}, err
	}
	cursorEventTime, cursorID := eventCursorValues(query.Cursor())
	rows, err := queriesFor(ctx, r.queries).ListUsageEvents(ctx, postgresdb.ListUsageEventsParams{
		WorkspaceID:     workspaceID,
		Subject:         eventStringValue(query.Subject()),
		MeterName:       eventStringValue(query.MeterName()),
		FromTime:        eventTimeValue(query.From()),
		ToTime:          eventTimeValue(query.To()),
		CursorEventTime: cursorEventTime,
		CursorID:        cursorID,
		Limit:           int32(query.Limit() + 1),
	})
	if err != nil {
		return domainusage.EventPage{}, err
	}

	events := make([]domainusage.Event, 0, len(rows))
	for _, row := range rows {
		event, err := eventFromFields(row.ID, row.IdempotencyKey, row.Subject, row.MeterName, row.Quantity, row.EventTime, row.ReceivedAt, row.Metadata, nil)
		if err != nil {
			return domainusage.EventPage{}, err
		}
		events = append(events, event)
	}

	return domainusage.NewEventPage(events, query.Limit()), nil
}

func (r *UsageRepository) CountEvents(ctx context.Context) (int, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return 0, err
	}
	count, err := queriesFor(ctx, r.queries).CountUsageEvents(ctx, workspaceID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *UsageRepository) FindMeterStats(ctx context.Context) ([]domainusage.MeterStats, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListUsageMeterStats(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	stats := make([]domainusage.MeterStats, 0, len(rows))
	for _, row := range rows {
		stat, err := meterStatsFromFields(row.MeterName, row.UsageEvents, row.LastEventAt)
		if err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}
	return stats, nil
}

func (r *UsageRepository) FindSubjectStats(ctx context.Context, query domainusage.SubjectStatsQuery) ([]domainusage.SubjectStats, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	cursorLastEventAt, cursorSubject := subjectStatsCursorValues(query)
	rows, err := queriesFor(ctx, r.queries).ListUsageSubjectStats(ctx, postgresdb.ListUsageSubjectStatsParams{
		WorkspaceID:       workspaceID,
		CursorLastEventAt: cursorLastEventAt,
		CursorSubject:     cursorSubject,
		Limit:             int32(query.Limit()),
	})
	if err != nil {
		return nil, err
	}

	stats := make([]domainusage.SubjectStats, 0, len(rows))
	for _, row := range rows {
		stat, err := subjectStatsFromFields(row.Subject, row.UsageEvents, row.Meters, row.LastEventAt)
		if err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}
	return stats, nil
}

func (r *UsageRepository) TryPruneLock(ctx context.Context) (bool, error) {
	return queriesFor(ctx, r.queries).TryPruneLock(ctx, pruneAdvisoryLockKey)
}

func (r *UsageRepository) PruneEvents(ctx context.Context, query domainusage.PruneQuery) (int, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return 0, err
	}
	sourceCount, err := r.rollUpPrunableEvents(ctx, workspaceID, query)
	if err != nil {
		return 0, err
	}
	total := 0
	for {
		deleted, err := queriesFor(ctx, r.queries).PruneUsageEventsBatch(ctx, postgresdb.PruneUsageEventsBatchParams{
			WorkspaceID: workspaceID,
			MeterName:   query.MeterName(),
			EventTime:   formatTime(query.Before()),
			Limit:       int32(pruneDeleteBatchSize),
		})
		if err != nil {
			return 0, err
		}

		total += int(deleted)
		if deleted < pruneDeleteBatchSize {
			if total > 0 {
				if err := queriesFor(ctx, r.queries).IncrementWorkspaceUsageEvents(ctx, postgresdb.IncrementWorkspaceUsageEventsParams{
					WorkspaceID: workspaceID,
					Delta:       -int64(total),
					UpdatedAt:   formatTime(time.Now().UTC()),
				}); err != nil {
					return 0, err
				}
			}
			if total != sourceCount {
				return 0, fmt.Errorf("usage rollup verification failed: materialized %d events but deleted %d", sourceCount, total)
			}
			return total, nil
		}
	}
}

func (r *UsageRepository) CountPrunableEvents(ctx context.Context, query domainusage.PruneQuery) (int, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return 0, err
	}
	count, err := queriesFor(ctx, r.queries).CountPrunableUsageEvents(ctx, postgresdb.CountPrunableUsageEventsParams{
		WorkspaceID: workspaceID,
		MeterName:   query.MeterName(),
		EventTime:   formatTime(query.Before()),
	})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *UsageRepository) SavePruneRun(ctx context.Context, run domainusage.PruneRun) (domainusage.PruneRun, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.PruneRun{}, err
	}
	meters, err := marshalPruneRunMeters(run.Meters())
	if err != nil {
		return domainusage.PruneRun{}, err
	}

	err = queriesFor(ctx, r.queries).SaveUsagePruneRun(ctx, postgresdb.SaveUsagePruneRunParams{
		ID:          run.ID(),
		WorkspaceID: workspaceID,
		DryRun:      int32(boolInt(run.DryRun())),
		Deleted:     int32(run.Deleted()),
		Meters:      meters,
		CreatedAt:   formatTime(run.CreatedAt()),
	})
	if err != nil {
		return domainusage.PruneRun{}, err
	}

	if err := queriesFor(ctx, r.queries).IncrementWorkspacePruneRuns(ctx, postgresdb.IncrementWorkspacePruneRunsParams{
		WorkspaceID: workspaceID,
		Delta:       1,
		UpdatedAt:   formatTime(time.Now().UTC()),
	}); err != nil {
		return domainusage.PruneRun{}, err
	}

	return run, nil
}

func (r *UsageRepository) FindPruneRuns(ctx context.Context, query domainusage.RunQuery) ([]domainusage.PruneRun, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	cursorCreatedAt, cursorID := runCursorValues(query)
	rows, err := queriesFor(ctx, r.queries).ListUsagePruneRuns(ctx, postgresdb.ListUsagePruneRunsParams{
		WorkspaceID:     workspaceID,
		CursorCreatedAt: cursorCreatedAt,
		CursorID:        cursorID,
		Limit:           int32(query.Limit()),
	})
	if err != nil {
		return nil, err
	}

	runs := make([]domainusage.PruneRun, 0, len(rows))
	for _, row := range rows {
		run, err := pruneRunFromFields(row.ID, row.DryRun, row.Deleted, row.Meters, row.CreatedAt)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func (r *UsageRepository) CountPruneRuns(ctx context.Context) (int, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return 0, err
	}
	count, err := queriesFor(ctx, r.queries).CountUsagePruneRuns(ctx, workspaceID)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *UsageRepository) SaveIngestionRun(ctx context.Context, run domainusage.IngestionRun) (domainusage.IngestionRun, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.IngestionRun{}, err
	}
	err = queriesFor(ctx, r.queries).SaveUsageIngestionRun(ctx, postgresdb.SaveUsageIngestionRunParams{
		ID:          run.ID(),
		WorkspaceID: workspaceID,
		Kind:        string(run.Kind()),
		Accepted:    int32(run.Accepted()),
		Duplicates:  int32(run.Duplicates()),
		Failed:      int32(run.Failed()),
		CreatedAt:   formatTime(run.CreatedAt()),
	})
	if err != nil {
		return domainusage.IngestionRun{}, err
	}

	return run, nil
}

func (r *UsageRepository) FindIngestionRuns(ctx context.Context, query domainusage.RunQuery) ([]domainusage.IngestionRun, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	cursorCreatedAt, cursorID := runCursorValues(query)
	rows, err := queriesFor(ctx, r.queries).ListUsageIngestionRuns(ctx, postgresdb.ListUsageIngestionRunsParams{
		WorkspaceID:     workspaceID,
		CursorCreatedAt: cursorCreatedAt,
		CursorID:        cursorID,
		Limit:           int32(query.Limit()),
	})
	if err != nil {
		return nil, err
	}

	runs := make([]domainusage.IngestionRun, 0, len(rows))
	for _, row := range rows {
		run, err := ingestionRunFromFields(row.ID, row.Kind, row.Accepted, row.Duplicates, row.Failed, row.CreatedAt)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func (r *UsageRepository) SaveExportJob(ctx context.Context, job domainusage.ExportJob) (domainusage.ExportJob, error) {
	err := queriesFor(ctx, r.queries).SaveUsageExportJob(ctx, postgresdb.SaveUsageExportJobParams{
		ID:           job.ID(),
		WorkspaceID:  job.WorkspaceID(),
		Kind:         string(job.Kind()),
		Status:       string(job.Status()),
		Format:       string(job.Format()),
		QueryJson:    job.QueryJSON(),
		Error:        job.ErrorMessage(),
		Attempts:     int32(job.Attempts()),
		LockedUntil:  exportJobTimeValue(job.LockedUntil()),
		ArtifactPath: job.ArtifactPath(),
		ArtifactSize: job.ArtifactSize(),
		CreatedAt:    formatTime(job.CreatedAt()),
		UpdatedAt:    formatTime(job.UpdatedAt()),
		CompletedAt:  exportJobTimeValue(job.CompletedAt()),
	})
	if err != nil {
		return domainusage.ExportJob{}, err
	}

	return job, nil
}

func (r *UsageRepository) FindExportJob(ctx context.Context, id string) (domainusage.ExportJob, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.ExportJob{}, err
	}
	row, err := queriesFor(ctx, r.queries).FindUsageExportJob(ctx, postgresdb.FindUsageExportJobParams{
		WorkspaceID: workspaceID,
		ID:          id,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainusage.ExportJob{}, domain.ErrNotFound
		}
		return domainusage.ExportJob{}, err
	}

	return exportJobFromFields(row.ID, row.WorkspaceID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, int(row.Attempts), row.LockedUntil, exportClaimToken(row.ClaimToken), row.ArtifactPath, row.ArtifactSize, row.CreatedAt, row.UpdatedAt, row.CompletedAt, row.ExpiredAt)
}

func (r *UsageRepository) FindExportJobs(ctx context.Context, query domainusage.RunQuery) ([]domainusage.ExportJob, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	cursorCreatedAt, cursorID := runCursorValues(query)
	rows, err := queriesFor(ctx, r.queries).ListUsageExportJobs(ctx, postgresdb.ListUsageExportJobsParams{
		WorkspaceID:     workspaceID,
		CursorCreatedAt: cursorCreatedAt,
		CursorID:        cursorID,
		Limit:           int32(query.Limit()),
	})
	if err != nil {
		return nil, err
	}

	jobs := make([]domainusage.ExportJob, 0, len(rows))
	for _, row := range rows {
		job, err := exportJobFromFields(row.ID, row.WorkspaceID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, int(row.Attempts), row.LockedUntil, exportClaimToken(row.ClaimToken), row.ArtifactPath, row.ArtifactSize, row.CreatedAt, row.UpdatedAt, row.CompletedAt, row.ExpiredAt)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (r *UsageRepository) FindExpiredExportJobs(ctx context.Context, expiredBefore time.Time, limit int) ([]domainusage.ExportJob, error) {
	rows, err := queriesFor(ctx, r.queries).ListExpiredUsageExportJobs(ctx, postgresdb.ListExpiredUsageExportJobsParams{ExpiredBefore: formatTime(expiredBefore), Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	jobs := make([]domainusage.ExportJob, 0, len(rows))
	for _, row := range rows {
		job, err := exportJobFromFields(row.ID, row.WorkspaceID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, int(row.Attempts), row.LockedUntil, exportClaimToken(row.ClaimToken), row.ArtifactPath, row.ArtifactSize, row.CreatedAt, row.UpdatedAt, row.CompletedAt, row.ExpiredAt)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (r *UsageRepository) ExpireExportJob(ctx context.Context, id string, expiredAt time.Time) (bool, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return false, err
	}
	rows, err := queriesFor(ctx, r.queries).ExpireUsageExportJob(ctx, postgresdb.ExpireUsageExportJobParams{ExpiredAt: formatTime(expiredAt), ID: id, WorkspaceID: workspaceID})
	return rows > 0, err
}

func (r *UsageRepository) SaveExportCleanupRun(ctx context.Context, run domainusage.ExportCleanupRun) (domainusage.ExportCleanupRun, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.ExportCleanupRun{}, err
	}
	if workspaceID != run.WorkspaceID() {
		return domainusage.ExportCleanupRun{}, domain.ErrInvalidInput
	}
	publicID, err := uuid.Parse(run.ID())
	if err != nil {
		return domainusage.ExportCleanupRun{}, err
	}
	err = queriesFor(ctx, r.queries).SaveUsageExportCleanupRun(ctx, postgresdb.SaveUsageExportCleanupRunParams{PublicID: publicID, WorkspaceID: workspaceID, ExpiredBefore: formatTime(run.ExpiredBefore()), FilesDeleted: int32(run.FilesDeleted()), BytesReclaimed: run.BytesReclaimed(), Failures: int32(run.Failures()), CreatedAt: formatTime(run.CreatedAt())})
	if err != nil {
		return domainusage.ExportCleanupRun{}, err
	}
	return run, nil
}

func (r *UsageRepository) ClaimExportJob(ctx context.Context, now time.Time, lockedUntil time.Time, claimToken string, maxAttempts int) (domainusage.ExportJob, error) {
	parsedToken, err := uuid.Parse(claimToken)
	if err != nil {
		return domainusage.ExportJob{}, err
	}
	row, err := queriesFor(ctx, r.queries).ClaimUsageExportJob(ctx, postgresdb.ClaimUsageExportJobParams{
		Now:         formatTime(now),
		LockedUntil: formatTime(lockedUntil),
		ClaimToken:  parsedToken,
		MaxAttempts: int32(maxAttempts),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainusage.ExportJob{}, domain.ErrNotFound
		}
		return domainusage.ExportJob{}, err
	}

	return exportJobFromFields(row.ID, row.WorkspaceID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, int(row.Attempts), row.LockedUntil, exportClaimToken(row.ClaimToken), row.ArtifactPath, row.ArtifactSize, row.CreatedAt, row.UpdatedAt, row.CompletedAt, row.ExpiredAt)
}

func (r *UsageRepository) RenewExportJobLease(ctx context.Context, id string, claimToken string, lockedUntil time.Time, now time.Time) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	parsedToken, err := uuid.Parse(claimToken)
	if err != nil {
		return err
	}
	rows, err := queriesFor(ctx, r.queries).RenewUsageExportJobLease(ctx, postgresdb.RenewUsageExportJobLeaseParams{LockedUntil: formatTime(lockedUntil), Now: formatTime(now), ID: id, WorkspaceID: workspaceID, ClaimToken: parsedToken})
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UsageRepository) CompleteExportJob(ctx context.Context, id string, claimToken string, artifactPath string, artifactSize int64, completedAt time.Time) (domainusage.ExportJob, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.ExportJob{}, err
	}
	parsedToken, err := uuid.Parse(claimToken)
	if err != nil {
		return domainusage.ExportJob{}, err
	}
	row, err := queriesFor(ctx, r.queries).CompleteUsageExportJob(ctx, postgresdb.CompleteUsageExportJobParams{
		ID:           id,
		WorkspaceID:  workspaceID,
		ArtifactPath: artifactPath,
		ArtifactSize: artifactSize,
		CompletedAt:  formatTime(completedAt),
		ClaimToken:   parsedToken,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainusage.ExportJob{}, domain.ErrNotFound
		}
		return domainusage.ExportJob{}, err
	}

	return exportJobFromFields(row.ID, row.WorkspaceID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, int(row.Attempts), row.LockedUntil, exportClaimToken(row.ClaimToken), row.ArtifactPath, row.ArtifactSize, row.CreatedAt, row.UpdatedAt, row.CompletedAt, row.ExpiredAt)
}

func (r *UsageRepository) FailExportJob(ctx context.Context, id string, claimToken string, errorMessage string, failedAt time.Time) (domainusage.ExportJob, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.ExportJob{}, err
	}
	parsedToken, err := uuid.Parse(claimToken)
	if err != nil {
		return domainusage.ExportJob{}, err
	}
	row, err := queriesFor(ctx, r.queries).FailUsageExportJob(ctx, postgresdb.FailUsageExportJobParams{
		ID:          id,
		WorkspaceID: workspaceID,
		Error:       errorMessage,
		FailedAt:    formatTime(failedAt),
		ClaimToken:  parsedToken,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainusage.ExportJob{}, domain.ErrNotFound
		}
		return domainusage.ExportJob{}, err
	}

	return exportJobFromFields(row.ID, row.WorkspaceID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, int(row.Attempts), row.LockedUntil, exportClaimToken(row.ClaimToken), row.ArtifactPath, row.ArtifactSize, row.CreatedAt, row.UpdatedAt, row.CompletedAt, row.ExpiredAt)
}

func (r *UsageRepository) CancelExportJob(ctx context.Context, id string, canceledAt time.Time) (domainusage.ExportJob, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.ExportJob{}, err
	}
	row, err := queriesFor(ctx, r.queries).CancelUsageExportJob(ctx, postgresdb.CancelUsageExportJobParams{
		ID:          id,
		WorkspaceID: workspaceID,
		CanceledAt:  formatTime(canceledAt),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainusage.ExportJob{}, domain.ErrNotFound
		}
		return domainusage.ExportJob{}, err
	}

	return exportJobFromFields(row.ID, row.WorkspaceID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, int(row.Attempts), row.LockedUntil, exportClaimToken(row.ClaimToken), row.ArtifactPath, row.ArtifactSize, row.CreatedAt, row.UpdatedAt, row.CompletedAt, row.ExpiredAt)
}

func (r *UsageRepository) RetryExportJob(ctx context.Context, id string, retriedAt time.Time) (domainusage.ExportJob, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.ExportJob{}, err
	}
	row, err := queriesFor(ctx, r.queries).RetryUsageExportJob(ctx, postgresdb.RetryUsageExportJobParams{
		ID:          id,
		WorkspaceID: workspaceID,
		RetriedAt:   formatTime(retriedAt),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainusage.ExportJob{}, domain.ErrNotFound
		}
		return domainusage.ExportJob{}, err
	}

	return exportJobFromFields(row.ID, row.WorkspaceID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, int(row.Attempts), row.LockedUntil, exportClaimToken(row.ClaimToken), row.ArtifactPath, row.ArtifactSize, row.CreatedAt, row.UpdatedAt, row.CompletedAt, row.ExpiredAt)
}

func (r *UsageRepository) findByIdempotencyKey(ctx context.Context, key string) (domainusage.Event, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.Event{}, err
	}
	event, err := queriesFor(ctx, r.queries).FindUsageEventByIdempotencyKey(ctx, postgresdb.FindUsageEventByIdempotencyKeyParams{
		WorkspaceID:    workspaceID,
		IdempotencyKey: key,
	})
	return eventFromFields(event.ID, event.IdempotencyKey, event.Subject, event.MeterName, event.Quantity, event.EventTime, event.ReceivedAt, event.Metadata, err)
}

func (r *UsageRepository) findByID(ctx context.Context, id string) (domainusage.Event, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.Event{}, err
	}
	event, err := queriesFor(ctx, r.queries).FindUsageEventByID(ctx, postgresdb.FindUsageEventByIDParams{
		WorkspaceID: workspaceID,
		ID:          id,
	})
	return eventFromFields(event.ID, event.IdempotencyKey, event.Subject, event.MeterName, event.Quantity, event.EventTime, event.ReceivedAt, event.Metadata, err)
}

func (r *UsageRepository) findBulk(ctx context.Context, idempotencyKey string) (domainusage.BulkSaveResult, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.BulkSaveResult{}, err
	}
	response, err := queriesFor(ctx, r.queries).FindBulkUsageIngestion(ctx, postgresdb.FindBulkUsageIngestionParams{
		WorkspaceID:    workspaceID,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return domainusage.BulkSaveResult{}, err
	}

	return unmarshalBulkResult(response)
}

func subjectStatsCursorValues(query domainusage.SubjectStatsQuery) (sql.NullString, sql.NullString) {
	if !query.HasCursor() {
		return sql.NullString{}, sql.NullString{}
	}
	return sql.NullString{String: formatTime(query.LastEventAt()), Valid: true}, sql.NullString{String: query.Subject(), Valid: true}
}

func eventStringValue(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func eventTimeValue(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(value), Valid: true}
}

func eventCursorValues(cursor domainusage.EventCursor) (sql.NullString, sql.NullString) {
	if cursor.IsZero() {
		return sql.NullString{}, sql.NullString{}
	}
	return sql.NullString{String: formatTime(cursor.EventTime()), Valid: true}, sql.NullString{String: cursor.ID(), Valid: true}
}

func meterStatsFromFields(meterName string, usageEvents int64, lastEventAtText string) (domainusage.MeterStats, error) {
	lastEventAt, err := time.Parse(time.RFC3339Nano, lastEventAtText)
	if err != nil {
		return domainusage.MeterStats{}, err
	}
	return domainusage.NewMeterStats(meterName, int(usageEvents), lastEventAt), nil
}

func subjectStatsFromFields(subject string, usageEvents int64, meters int64, lastEventAtText string) (domainusage.SubjectStats, error) {
	lastEventAt, err := time.Parse(time.RFC3339Nano, lastEventAtText)
	if err != nil {
		return domainusage.SubjectStats{}, err
	}
	return domainusage.NewSubjectStats(subject, int(usageEvents), int(meters), lastEventAt), nil
}

func runCursorValues(query domainusage.RunQuery) (sql.NullString, sql.NullString) {
	if !query.HasCursor() {
		return sql.NullString{}, sql.NullString{}
	}
	return sql.NullString{String: formatTime(query.CreatedAt()), Valid: true}, sql.NullString{String: query.ID(), Valid: true}
}

func exportJobTimeValue(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(value), Valid: true}
}

func ingestionRunFromFields(id string, kind string, accepted int32, duplicates int32, failed int32, createdAtText string) (domainusage.IngestionRun, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtText)
	if err != nil {
		return domainusage.IngestionRun{}, err
	}

	return domainusage.NewIngestionRun(id, domainusage.IngestionKind(kind), int(accepted), int(duplicates), int(failed), createdAt)
}

func exportJobFromFields(id string, workspaceID string, kind string, status string, format string, queryJSON string, errorMessage string, attempts int, lockedUntilText sql.NullString, claimToken string, artifactPath string, artifactSize int64, createdAtText string, updatedAtText string, completedAtText sql.NullString, expiredAtText sql.NullString) (domainusage.ExportJob, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtText)
	if err != nil {
		return domainusage.ExportJob{}, err
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, updatedAtText)
	if err != nil {
		return domainusage.ExportJob{}, err
	}
	completedAt := time.Time{}
	if completedAtText.Valid {
		completedAt, err = time.Parse(time.RFC3339Nano, completedAtText.String)
		if err != nil {
			return domainusage.ExportJob{}, err
		}
	}
	lockedUntil := time.Time{}
	if lockedUntilText.Valid {
		lockedUntil, err = time.Parse(time.RFC3339Nano, lockedUntilText.String)
		if err != nil {
			return domainusage.ExportJob{}, err
		}
	}
	expiredAt := time.Time{}
	if expiredAtText.Valid {
		expiredAt, err = time.Parse(time.RFC3339Nano, expiredAtText.String)
		if err != nil {
			return domainusage.ExportJob{}, err
		}
	}

	return domainusage.NewExportJob(
		id,
		workspaceID,
		domainusage.ExportJobKind(kind),
		domainusage.ExportJobStatus(status),
		domainusage.ExportJobFormat(format),
		queryJSON,
		errorMessage,
		attempts,
		lockedUntil,
		claimToken,
		artifactPath,
		artifactSize,
		createdAt,
		updatedAt,
		completedAt,
		expiredAt,
	)
}

func exportClaimToken(value uuid.NullUUID) string {
	if !value.Valid {
		return ""
	}
	return value.UUID.String()
}

func eventFromFields(id string, idempotencyKey sql.NullString, subject string, meterName string, quantity float64, eventTimeText string, receivedAtText string, metadataJSON json.RawMessage, err error) (domainusage.Event, error) {
	if err != nil {
		return domainusage.Event{}, err
	}

	eventTime, err := time.Parse(time.RFC3339Nano, eventTimeText)
	if err != nil {
		return domainusage.Event{}, err
	}
	receivedAt, err := time.Parse(time.RFC3339Nano, receivedAtText)
	if err != nil {
		return domainusage.Event{}, err
	}

	metadata := map[string]any{}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
			return domainusage.Event{}, err
		}
	}

	return domainusage.NewEvent(
		id,
		idempotencyKey.String,
		subject,
		meterName,
		quantity,
		eventTime,
		receivedAt,
		metadata,
	)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func pruneRunFromFields(id string, dryRun int32, deleted int32, metersText string, createdAtText string) (domainusage.PruneRun, error) {
	meters, err := unmarshalPruneRunMeters(metersText)
	if err != nil {
		return domainusage.PruneRun{}, err
	}
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtText)
	if err != nil {
		return domainusage.PruneRun{}, err
	}

	return domainusage.NewPruneRun(id, dryRun != 0, int(deleted), meters, createdAt)
}

func scanEvent(scanner interface {
	Scan(dest ...any) error
}) (domainusage.Event, error) {
	var id string
	var idempotencyKey sql.NullString
	var subject string
	var meterName string
	var quantity float64
	var eventTimeText string
	var receivedAtText string
	var metadataText string

	if err := scanner.Scan(&id, &idempotencyKey, &subject, &meterName, &quantity, &eventTimeText, &receivedAtText, &metadataText); err != nil {
		return domainusage.Event{}, err
	}

	return eventFromFields(id, idempotencyKey, subject, meterName, quantity, eventTimeText, receivedAtText, json.RawMessage(metadataText), nil)
}

func marshalPruneRunMeters(meters []domainusage.PruneRunMeter) (string, error) {
	snapshots := make([]pruneRunMeterSnapshot, 0, len(meters))
	for _, meter := range meters {
		snapshots = append(snapshots, pruneRunMeterSnapshot{
			MeterName: meter.MeterName(),
			Before:    formatTime(meter.Before()),
			Deleted:   meter.Deleted(),
		})
	}

	payload, err := json.Marshal(snapshots)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func unmarshalPruneRunMeters(payload string) ([]domainusage.PruneRunMeter, error) {
	snapshots := []pruneRunMeterSnapshot{}
	if err := json.Unmarshal([]byte(payload), &snapshots); err != nil {
		return nil, err
	}

	meters := make([]domainusage.PruneRunMeter, 0, len(snapshots))
	for _, snapshot := range snapshots {
		before, err := time.Parse(time.RFC3339Nano, snapshot.Before)
		if err != nil {
			return nil, err
		}
		meter, err := domainusage.NewPruneRunMeter(snapshot.MeterName, before, snapshot.Deleted)
		if err != nil {
			return nil, err
		}
		meters = append(meters, meter)
	}

	return meters, nil
}

func marshalEvents(events []domainusage.Event) (string, error) {
	snapshots := make([]eventSnapshot, 0, len(events))
	for _, event := range events {
		snapshots = append(snapshots, eventSnapshot{
			ID:             event.ID(),
			IdempotencyKey: event.IdempotencyKey(),
			Subject:        event.Subject(),
			MeterName:      event.MeterName(),
			Quantity:       event.Quantity(),
			EventTime:      formatTime(event.EventTime()),
			ReceivedAt:     formatTime(event.ReceivedAt()),
			Metadata:       event.Metadata(),
		})
	}

	payload, err := json.Marshal(snapshots)
	if err != nil {
		return "", err
	}

	return string(payload), nil
}

func marshalBulkResult(result domainusage.BulkSaveResult) (string, error) {
	payload, err := json.Marshal(bulkSnapshot{
		Accepted:   eventSnapshots(result.Accepted()),
		Duplicates: eventSnapshots(result.Duplicates()),
	})
	if err != nil {
		return "", err
	}

	return string(payload), nil
}

func unmarshalBulkResult(payload string) (domainusage.BulkSaveResult, error) {
	var snapshot bulkSnapshot
	if err := json.Unmarshal([]byte(payload), &snapshot); err == nil && (snapshot.Accepted != nil || snapshot.Duplicates != nil) {
		accepted, err := eventsFromSnapshots(snapshot.Accepted)
		if err != nil {
			return domainusage.BulkSaveResult{}, err
		}
		duplicates, err := eventsFromSnapshots(snapshot.Duplicates)
		if err != nil {
			return domainusage.BulkSaveResult{}, err
		}
		return domainusage.NewBulkSaveResult(accepted, duplicates), nil
	}

	events, err := unmarshalEvents(payload)
	if err != nil {
		return domainusage.BulkSaveResult{}, err
	}
	return domainusage.NewBulkSaveResult(events, nil), nil
}

func unmarshalEvents(payload string) ([]domainusage.Event, error) {
	snapshots := []eventSnapshot{}
	if err := json.Unmarshal([]byte(payload), &snapshots); err != nil {
		return nil, err
	}

	return eventsFromSnapshots(snapshots)
}

func eventSnapshots(events []domainusage.Event) []eventSnapshot {
	snapshots := make([]eventSnapshot, 0, len(events))
	for _, event := range events {
		snapshots = append(snapshots, eventSnapshot{
			ID:             event.ID(),
			IdempotencyKey: event.IdempotencyKey(),
			Subject:        event.Subject(),
			MeterName:      event.MeterName(),
			Quantity:       event.Quantity(),
			EventTime:      formatTime(event.EventTime()),
			ReceivedAt:     formatTime(event.ReceivedAt()),
			Metadata:       event.Metadata(),
		})
	}
	return snapshots
}

func eventsFromSnapshots(snapshots []eventSnapshot) ([]domainusage.Event, error) {
	events := make([]domainusage.Event, 0, len(snapshots))
	for _, snapshot := range snapshots {
		eventTime, err := time.Parse(time.RFC3339Nano, snapshot.EventTime)
		if err != nil {
			return nil, err
		}
		receivedAt, err := time.Parse(time.RFC3339Nano, snapshot.ReceivedAt)
		if err != nil {
			return nil, err
		}

		event, err := domainusage.NewEvent(
			snapshot.ID,
			snapshot.IdempotencyKey,
			snapshot.Subject,
			snapshot.MeterName,
			snapshot.Quantity,
			eventTime,
			receivedAt,
			snapshot.Metadata,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, nil
}
