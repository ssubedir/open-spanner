package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

var metadataKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_]+(\.[A-Za-z0-9_]+)*$`)

var errBulkReplay = errors.New("bulk ingestion already exists")

const (
	pruneAdvisoryLockKey = int64(0x4f535052554e45)
	pruneDeleteBatchSize = 1000
)

type UsageRepository struct {
	store *Store
}

type eventStore interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
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
	return &UsageRepository{store: store}
}

func (r *UsageRepository) Save(ctx context.Context, event domainusage.Event) (domainusage.Event, error) {
	return r.save(ctx, r.store, event)
}

func (r *UsageRepository) SaveBulk(ctx context.Context, idempotencyKey string, events []domainusage.Event) (domainusage.BulkSaveResult, error) {
	if idempotencyKey != "" {
		existing, err := r.findBulk(ctx, idempotencyKey)
		if err == nil {
			return existing, nil
		}
		if err != sql.ErrNoRows {
			return domainusage.BulkSaveResult{}, err
		}
	}

	var result domainusage.BulkSaveResult
	err := r.store.WithinTransaction(ctx, func(txCtx context.Context) error {
		accepted := make([]domainusage.Event, 0, len(events))
		duplicates := []domainusage.Event{}
		for _, event := range events {
			savedEvent, duplicate, err := r.saveWithDuplicate(txCtx, r.store, event)
			if err != nil {
				return err
			}
			if duplicate {
				duplicates = append(duplicates, savedEvent)
				continue
			}
			accepted = append(accepted, savedEvent)
		}

		result = domainusage.NewBulkSaveResult(accepted, duplicates)
		if idempotencyKey == "" {
			return nil
		}

		response, err := marshalBulkResult(result)
		if err != nil {
			return err
		}

		_, err = r.store.ExecContext(txCtx, `
INSERT INTO bulk_usage_ingestions (idempotency_key, response, created_at)
VALUES ($1, $2, $3)
`, idempotencyKey, response, formatTime(time.Now().UTC()))
		if err != nil {
			if isUniqueConstraint(err) {
				existing, findErr := r.findBulk(ctx, idempotencyKey)
				if findErr != nil {
					return findErr
				}
				result = existing
				return errBulkReplay
			}
			return err
		}

		return nil
	})
	if errors.Is(err, errBulkReplay) {
		return result, nil
	}
	if err != nil {
		return domainusage.BulkSaveResult{}, err
	}

	return result, nil
}

func (r *UsageRepository) save(ctx context.Context, store eventStore, event domainusage.Event) (domainusage.Event, error) {
	saved, _, err := r.saveWithDuplicate(ctx, store, event)
	return saved, err
}

func (r *UsageRepository) saveWithDuplicate(ctx context.Context, store eventStore, event domainusage.Event) (domainusage.Event, bool, error) {
	if _, err := r.findByID(ctx, store, event.ID()); err == nil {
		return domainusage.Event{}, false, domain.ErrConflict
	} else if err != sql.ErrNoRows {
		return domainusage.Event{}, false, err
	}

	if event.IdempotencyKey() != "" {
		existing, err := r.findByIdempotencyKey(ctx, store, event.IdempotencyKey())
		if err == nil {
			return existing, true, nil
		}
		if err != sql.ErrNoRows {
			return domainusage.Event{}, false, err
		}
	}

	metadata, err := json.Marshal(event.Metadata())
	if err != nil {
		return domainusage.Event{}, false, err
	}

	_, err = store.ExecContext(ctx, `
INSERT INTO usage_events (
	id,
	idempotency_key,
	subject,
	meter_name,
	quantity,
	event_time,
	received_at,
	metadata
) VALUES ($1, NULLIF($2, ''), $3, $4, $5, $6, $7, $8)
`, event.ID(), event.IdempotencyKey(), event.Subject(), event.MeterName(), event.Quantity(), formatTime(event.EventTime()), formatTime(event.ReceivedAt()), string(metadata))
	if err != nil {
		if isUniqueConstraint(err) && event.IdempotencyKey() != "" {
			existing, findErr := r.findByIdempotencyKey(ctx, store, event.IdempotencyKey())
			return existing, true, findErr
		}
		if isUniqueConstraint(err) {
			return domainusage.Event{}, false, errors.Join(domain.ErrConflict, err)
		}
		return domainusage.Event{}, false, err
	}

	return event, false, nil
}

func (r *UsageRepository) Query(ctx context.Context, query domainusage.Query) ([]domainusage.Bucket, error) {
	sqlQuery := strings.Builder{}
	sqlQuery.WriteString(`
SELECT id, idempotency_key, subject, meter_name, quantity, event_time, received_at, metadata
FROM usage_events
`)
	args := []any{}
	sqlQuery.WriteString("WHERE subject = " + bindArg(&args, query.Subject()) + "\n")
	sqlQuery.WriteString("\tAND meter_name = " + bindArg(&args, query.MeterName()) + "\n")
	sqlQuery.WriteString("\tAND event_time >= " + bindArg(&args, formatTime(query.From())) + "\n")
	sqlQuery.WriteString("\tAND event_time < " + bindArg(&args, formatTime(query.To())) + "\n")
	filterSQL, err := filterWhereSQL(query.Filter(), &args)
	if err != nil {
		return nil, err
	}
	if filterSQL != "" {
		sqlQuery.WriteString(" AND ")
		sqlQuery.WriteString(filterSQL)
		sqlQuery.WriteString("\n")
	}
	sqlQuery.WriteString(`
ORDER BY event_time
`)

	rows, err := r.store.query(ctx, sqlQuery.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []domainusage.Event{}
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return domainusage.AggregateEvents(query, events), nil
}

func (r *UsageRepository) FindEvents(ctx context.Context, query domainusage.EventQuery) (domainusage.EventPage, error) {
	sqlQuery := strings.Builder{}
	sqlQuery.WriteString(`
SELECT id, idempotency_key, subject, meter_name, quantity, event_time, received_at, metadata
FROM usage_events
WHERE 1 = 1
`)
	args := []any{}
	if query.Subject() != "" {
		sqlQuery.WriteString(" AND subject = " + bindArg(&args, query.Subject()) + "\n")
	}
	if query.MeterName() != "" {
		sqlQuery.WriteString(" AND meter_name = " + bindArg(&args, query.MeterName()) + "\n")
	}
	if !query.From().IsZero() {
		sqlQuery.WriteString(" AND event_time >= " + bindArg(&args, formatTime(query.From())) + "\n")
	}
	if !query.To().IsZero() {
		sqlQuery.WriteString(" AND event_time < " + bindArg(&args, formatTime(query.To())) + "\n")
	}
	filterSQL, err := filterWhereSQL(query.Filter(), &args)
	if err != nil {
		return domainusage.EventPage{}, err
	}
	if filterSQL != "" {
		sqlQuery.WriteString(" AND ")
		sqlQuery.WriteString(filterSQL)
		sqlQuery.WriteString("\n")
	}
	if !query.Cursor().IsZero() {
		cursorTime := formatTime(query.Cursor().EventTime())
		cursorTimeRef := bindArg(&args, cursorTime)
		idRef := bindArg(&args, query.Cursor().ID())
		sqlQuery.WriteString(" AND (event_time < " + cursorTimeRef + " OR (event_time = " + cursorTimeRef + " AND id < " + idRef + "))\n")
	}
	sqlQuery.WriteString("ORDER BY event_time DESC, id DESC\nLIMIT " + bindArg(&args, query.Limit()+1))

	rows, err := r.store.query(ctx, sqlQuery.String(), args...)
	if err != nil {
		return domainusage.EventPage{}, err
	}
	defer rows.Close()

	events := []domainusage.Event{}
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return domainusage.EventPage{}, err
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return domainusage.EventPage{}, err
	}

	return domainusage.NewEventPage(events, query.Limit()), nil
}

func filterWhereSQL(filter domainusage.Filter, args *[]any) (string, error) {
	if filter.IsZero() {
		return "", nil
	}

	switch filter.Type() {
	case domainusage.FilterTypeGroup:
		parts := []string{}
		for _, rule := range filter.Rules() {
			part, err := filterWhereSQL(rule, args)
			if err != nil {
				return "", err
			}
			if part == "" {
				continue
			}
			parts = append(parts, "("+part+")")
		}
		if len(parts) == 0 {
			return "", nil
		}
		joiner := " AND "
		if filter.GroupOp() == domainusage.FilterGroupOr {
			joiner = " OR "
		}
		return strings.Join(parts, joiner), nil
	case domainusage.FilterTypeCondition:
		return conditionWhereSQL(filter, args)
	default:
		return "", nil
	}
}

func conditionWhereSQL(filter domainusage.Filter, args *[]any) (string, error) {
	fieldSQL, valueKind, err := filterFieldSQL(filter.Field())
	if err != nil {
		return "", err
	}

	op := filter.ConditionOp()
	if op == domainusage.FilterOpExists {
		return fieldSQL + " IS NOT NULL", nil
	}

	switch op {
	case domainusage.FilterOpEqual, domainusage.FilterOpNotEqual, domainusage.FilterOpGreaterThan, domainusage.FilterOpGreaterThanOrEqual, domainusage.FilterOpLessThan, domainusage.FilterOpLessThanOrEqual:
		value, err := sqlFilterValue(filter.Value(), valueKind)
		if err != nil {
			return "", err
		}
		return fieldSQL + " " + sqlOperator(op) + " " + bindArg(args, value), nil
	case domainusage.FilterOpIn:
		values, ok := filter.Value().([]any)
		if !ok || len(values) == 0 {
			return "", fmt.Errorf("invalid in filter value")
		}
		placeholders := make([]string, 0, len(values))
		for _, raw := range values {
			value, err := sqlFilterValue(raw, valueKind)
			if err != nil {
				return "", err
			}
			placeholders = append(placeholders, bindArg(args, value))
		}
		return fieldSQL + " IN (" + strings.Join(placeholders, ", ") + ")", nil
	case domainusage.FilterOpContains:
		value, err := sqlFilterValue(filter.Value(), "text")
		if err != nil {
			return "", err
		}
		return "CAST(" + fieldSQL + " AS TEXT) LIKE " + bindArg(args, "%"+fmt.Sprint(value)+"%"), nil
	default:
		return "", fmt.Errorf("unsupported filter operator %q", op)
	}
}

func sqlOperator(op domainusage.FilterConditionOp) string {
	switch op {
	case domainusage.FilterOpNotEqual:
		return "!="
	case domainusage.FilterOpGreaterThan:
		return ">"
	case domainusage.FilterOpGreaterThanOrEqual:
		return ">="
	case domainusage.FilterOpLessThan:
		return "<"
	case domainusage.FilterOpLessThanOrEqual:
		return "<="
	default:
		return "="
	}
}

func filterFieldSQL(field string) (string, string, error) {
	switch field {
	case "subject":
		return "subject", "text", nil
	case "meter":
		return "meter_name", "text", nil
	case "quantity":
		return "quantity", "number", nil
	case "timestamp", "event_time":
		return "event_time", "time", nil
	case "received_at":
		return "received_at", "time", nil
	case "idempotency_key":
		return "idempotency_key", "text", nil
	default:
		key := strings.TrimPrefix(field, "metadata.")
		if key == field || !metadataKeyPattern.MatchString(key) {
			return "", "", fmt.Errorf("unsupported filter field %q", field)
		}
		return "metadata::jsonb #>> " + postgresJSONPath(key), "any", nil
	}
}

func sqlFilterValue(value any, kind string) (any, error) {
	if kind == "time" {
		parsed, err := time.Parse(time.RFC3339Nano, fmt.Sprint(value))
		if err != nil {
			return nil, err
		}
		return formatTime(parsed), nil
	}
	return value, nil
}

func (r *UsageRepository) CountEvents(ctx context.Context) (int, error) {
	var count int
	if err := r.store.queryRow(ctx, `SELECT COUNT(*) FROM usage_events`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *UsageRepository) FindMeterStats(ctx context.Context) ([]domainusage.MeterStats, error) {
	rows, err := r.store.query(ctx, `
SELECT meter_name, COUNT(*), MAX(event_time)
FROM usage_events
GROUP BY meter_name
ORDER BY meter_name
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := []domainusage.MeterStats{}
	for rows.Next() {
		var meterName string
		var usageEvents int
		var lastEventAtText string
		if err := rows.Scan(&meterName, &usageEvents, &lastEventAtText); err != nil {
			return nil, err
		}
		lastEventAt, err := time.Parse(time.RFC3339Nano, lastEventAtText)
		if err != nil {
			return nil, err
		}
		stats = append(stats, domainusage.NewMeterStats(meterName, usageEvents, lastEventAt))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

func (r *UsageRepository) FindSubjectStats(ctx context.Context, query domainusage.SubjectStatsQuery) ([]domainusage.SubjectStats, error) {
	sqlQuery := strings.Builder{}
	sqlQuery.WriteString(`
SELECT subject, COUNT(*), COUNT(DISTINCT meter_name), MAX(event_time)
FROM usage_events
GROUP BY subject
`)
	args := []any{}
	if query.HasCursor() {
		cursorTime := formatTime(query.LastEventAt())
		cursorTimeRef := bindArg(&args, cursorTime)
		subjectRef := bindArg(&args, query.Subject())
		sqlQuery.WriteString("HAVING MAX(event_time) < " + cursorTimeRef + " OR (MAX(event_time) = " + cursorTimeRef + " AND subject > " + subjectRef + ")\n")
	}
	sqlQuery.WriteString(`ORDER BY MAX(event_time) DESC, subject ASC
LIMIT ` + bindArg(&args, query.Limit()) + `
`)

	rows, err := r.store.query(ctx, sqlQuery.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := []domainusage.SubjectStats{}
	for rows.Next() {
		var subject string
		var usageEvents int
		var meters int
		var lastEventAtText string
		if err := rows.Scan(&subject, &usageEvents, &meters, &lastEventAtText); err != nil {
			return nil, err
		}
		lastEventAt, err := time.Parse(time.RFC3339Nano, lastEventAtText)
		if err != nil {
			return nil, err
		}
		stats = append(stats, domainusage.NewSubjectStats(subject, usageEvents, meters, lastEventAt))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

func (r *UsageRepository) TryPruneLock(ctx context.Context) (bool, error) {
	var locked bool
	if err := r.store.queryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, pruneAdvisoryLockKey).Scan(&locked); err != nil {
		return false, err
	}
	return locked, nil
}

func (r *UsageRepository) PruneEvents(ctx context.Context, query domainusage.PruneQuery) (int, error) {
	return r.pruneEvents(ctx, r.store, query)
}

func (r *UsageRepository) pruneEvents(ctx context.Context, store eventStore, query domainusage.PruneQuery) (int, error) {
	total := 0
	for {
		result, err := store.ExecContext(ctx, `
WITH deleted AS (
	SELECT id
	FROM usage_events
	WHERE meter_name = $1
		AND event_time < $2
	ORDER BY event_time ASC, id ASC
	LIMIT $3
)
DELETE FROM usage_events
WHERE id IN (SELECT id FROM deleted)
`, query.MeterName(), formatTime(query.Before()), pruneDeleteBatchSize)
		if err != nil {
			return 0, err
		}

		deleted, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		total += int(deleted)
		if deleted < pruneDeleteBatchSize {
			return total, nil
		}
	}
}

func (r *UsageRepository) CountPrunableEvents(ctx context.Context, query domainusage.PruneQuery) (int, error) {
	return r.countPrunableEvents(ctx, r.store, query)
}

func (r *UsageRepository) countPrunableEvents(ctx context.Context, store eventStore, query domainusage.PruneQuery) (int, error) {
	var count int
	err := store.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM usage_events
WHERE meter_name = $1
	AND event_time < $2
`, query.MeterName(), formatTime(query.Before())).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *UsageRepository) SavePruneRun(ctx context.Context, run domainusage.PruneRun) (domainusage.PruneRun, error) {
	return r.savePruneRun(ctx, r.store, run)
}

func (r *UsageRepository) savePruneRun(ctx context.Context, store eventStore, run domainusage.PruneRun) (domainusage.PruneRun, error) {
	meters, err := marshalPruneRunMeters(run.Meters())
	if err != nil {
		return domainusage.PruneRun{}, err
	}

	_, err = store.ExecContext(ctx, `
INSERT INTO usage_prune_runs (id, dry_run, deleted, meters, created_at)
VALUES ($1, $2, $3, $4, $5)
`, run.ID(), boolInt(run.DryRun()), run.Deleted(), meters, formatTime(run.CreatedAt()))
	if err != nil {
		return domainusage.PruneRun{}, err
	}

	return run, nil
}

func (r *UsageRepository) FindPruneRuns(ctx context.Context, query domainusage.RunQuery) ([]domainusage.PruneRun, error) {
	sqlQuery := strings.Builder{}
	sqlQuery.WriteString(`
SELECT id, dry_run, deleted, meters, created_at
FROM usage_prune_runs
WHERE 1 = 1
`)
	args := []any{}
	if query.HasCursor() {
		cursorTime := formatTime(query.CreatedAt())
		cursorTimeRef := bindArg(&args, cursorTime)
		idRef := bindArg(&args, query.ID())
		sqlQuery.WriteString(" AND (created_at < " + cursorTimeRef + " OR (created_at = " + cursorTimeRef + " AND id < " + idRef + "))\n")
	}
	sqlQuery.WriteString(`ORDER BY created_at DESC, id DESC
LIMIT ` + bindArg(&args, query.Limit()) + `
`)

	rows, err := r.store.query(ctx, sqlQuery.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := []domainusage.PruneRun{}
	for rows.Next() {
		run, err := scanPruneRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return runs, nil
}

func (r *UsageRepository) CountPruneRuns(ctx context.Context) (int, error) {
	var count int
	if err := r.store.queryRow(ctx, `SELECT COUNT(*) FROM usage_prune_runs`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *UsageRepository) SaveIngestionRun(ctx context.Context, run domainusage.IngestionRun) (domainusage.IngestionRun, error) {
	_, err := r.store.exec(ctx, `
INSERT INTO usage_ingestions (id, kind, accepted, duplicates, failed, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
`, run.ID(), string(run.Kind()), run.Accepted(), run.Duplicates(), run.Failed(), formatTime(run.CreatedAt()))
	if err != nil {
		return domainusage.IngestionRun{}, err
	}

	return run, nil
}

func (r *UsageRepository) FindIngestionRuns(ctx context.Context, query domainusage.RunQuery) ([]domainusage.IngestionRun, error) {
	sqlQuery := strings.Builder{}
	sqlQuery.WriteString(`
SELECT id, kind, accepted, duplicates, failed, created_at
FROM usage_ingestions
WHERE 1 = 1
`)
	args := []any{}
	if query.HasCursor() {
		cursorTime := formatTime(query.CreatedAt())
		cursorTimeRef := bindArg(&args, cursorTime)
		idRef := bindArg(&args, query.ID())
		sqlQuery.WriteString(" AND (created_at < " + cursorTimeRef + " OR (created_at = " + cursorTimeRef + " AND id < " + idRef + "))\n")
	}
	sqlQuery.WriteString(`ORDER BY created_at DESC, id DESC
LIMIT ` + bindArg(&args, query.Limit()) + `
`)

	rows, err := r.store.query(ctx, sqlQuery.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := []domainusage.IngestionRun{}
	for rows.Next() {
		run, err := scanIngestionRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return runs, nil
}

func (r *UsageRepository) findByIdempotencyKey(ctx context.Context, store eventStore, key string) (domainusage.Event, error) {
	row := store.QueryRowContext(ctx, `
SELECT id, idempotency_key, subject, meter_name, quantity, event_time, received_at, metadata
FROM usage_events
WHERE idempotency_key = $1
`, key)

	return scanEvent(row)
}

func (r *UsageRepository) findByID(ctx context.Context, store eventStore, id string) (domainusage.Event, error) {
	row := store.QueryRowContext(ctx, `
SELECT id, idempotency_key, subject, meter_name, quantity, event_time, received_at, metadata
FROM usage_events
WHERE id = $1
`, id)

	return scanEvent(row)
}

func postgresJSONPath(key string) string {
	return "'{" + strings.ReplaceAll(key, ".", ",") + "}'"
}

func (r *UsageRepository) findBulk(ctx context.Context, idempotencyKey string) (domainusage.BulkSaveResult, error) {
	var response string
	err := r.store.queryRow(ctx, `
SELECT response
FROM bulk_usage_ingestions
WHERE idempotency_key = $1
`, idempotencyKey).Scan(&response)
	if err != nil {
		return domainusage.BulkSaveResult{}, err
	}

	return unmarshalBulkResult(response)
}

func scanIngestionRun(scanner interface {
	Scan(dest ...any) error
}) (domainusage.IngestionRun, error) {
	var id string
	var kind string
	var accepted int
	var duplicates int
	var failed int
	var createdAtText string

	if err := scanner.Scan(&id, &kind, &accepted, &duplicates, &failed, &createdAtText); err != nil {
		return domainusage.IngestionRun{}, err
	}

	createdAt, err := time.Parse(time.RFC3339Nano, createdAtText)
	if err != nil {
		return domainusage.IngestionRun{}, err
	}

	return domainusage.NewIngestionRun(id, domainusage.IngestionKind(kind), accepted, duplicates, failed, createdAt)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func scanPruneRun(scanner interface {
	Scan(dest ...any) error
}) (domainusage.PruneRun, error) {
	var id string
	var dryRun int
	var deleted int
	var metersText string
	var createdAtText string

	if err := scanner.Scan(&id, &dryRun, &deleted, &metersText, &createdAtText); err != nil {
		return domainusage.PruneRun{}, err
	}

	meters, err := unmarshalPruneRunMeters(metersText)
	if err != nil {
		return domainusage.PruneRun{}, err
	}
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtText)
	if err != nil {
		return domainusage.PruneRun{}, err
	}

	return domainusage.NewPruneRun(id, dryRun != 0, deleted, meters, createdAt)
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

	eventTime, err := time.Parse(time.RFC3339Nano, eventTimeText)
	if err != nil {
		return domainusage.Event{}, err
	}
	receivedAt, err := time.Parse(time.RFC3339Nano, receivedAtText)
	if err != nil {
		return domainusage.Event{}, err
	}

	metadata := map[string]any{}
	if metadataText != "" {
		if err := json.Unmarshal([]byte(metadataText), &metadata); err != nil {
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
