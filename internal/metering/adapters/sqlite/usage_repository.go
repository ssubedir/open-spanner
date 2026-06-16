package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite/sqlitedb"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

var metadataKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_]+(\.[A-Za-z0-9_]+)*$`)

var errBulkReplay = errors.New("bulk ingestion already exists")

type UsageRepository struct {
	store   *Store
	queries *sqlitedb.Queries
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
	return &UsageRepository{store: store, queries: sqlitedb.New(store)}
}

func (r *UsageRepository) Save(ctx context.Context, event domainusage.Event) (domainusage.Event, error) {
	return r.save(ctx, event)
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
			savedEvent, duplicate, err := r.saveWithDuplicate(txCtx, event)
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

		err = queriesFor(txCtx, r.queries).SaveBulkUsageIngestion(txCtx, sqlitedb.SaveBulkUsageIngestionParams{
			IdempotencyKey: idempotencyKey,
			Response:       response,
			CreatedAt:      formatTime(time.Now().UTC()),
		})
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

func (r *UsageRepository) save(ctx context.Context, event domainusage.Event) (domainusage.Event, error) {
	saved, _, err := r.saveWithDuplicate(ctx, event)
	return saved, err
}

func (r *UsageRepository) saveWithDuplicate(ctx context.Context, event domainusage.Event) (domainusage.Event, bool, error) {
	if _, err := r.findByID(ctx, event.ID()); err == nil {
		return domainusage.Event{}, false, domain.ErrConflict
	} else if err != sql.ErrNoRows {
		return domainusage.Event{}, false, err
	}

	if event.IdempotencyKey() != "" {
		existing, err := r.findByIdempotencyKey(ctx, event.IdempotencyKey())
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

	err = queriesFor(ctx, r.queries).SaveUsageEvent(ctx, sqlitedb.SaveUsageEventParams{
		ID:             event.ID(),
		IdempotencyKey: event.IdempotencyKey(),
		Subject:        event.Subject(),
		MeterName:      event.MeterName(),
		Quantity:       event.Quantity(),
		EventTime:      formatTime(event.EventTime()),
		ReceivedAt:     formatTime(event.ReceivedAt()),
		Metadata:       string(metadata),
	})
	if err != nil {
		if isUniqueConstraint(err) && event.IdempotencyKey() != "" {
			existing, findErr := r.findByIdempotencyKey(ctx, event.IdempotencyKey())
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
	if query.Filter().IsZero() && len(query.Metadata()) == 0 && len(query.GroupByFields()) == 0 {
		return r.queryBuckets(ctx, query)
	}

	return r.queryDynamicBuckets(ctx, query)
}

func (r *UsageRepository) queryBuckets(ctx context.Context, query domainusage.Query) ([]domainusage.Bucket, error) {
	rows, err := queriesFor(ctx, r.queries).ListUsageBuckets(ctx, sqlitedb.ListUsageBucketsParams{
		Aggregation: string(query.Aggregation()),
		BucketSize:  string(query.BucketSize()),
		Limit:       int64(query.Limit()),
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
		bucketStart, err := time.Parse(time.RFC3339Nano, row.BucketStart)
		if err != nil {
			return nil, err
		}
		buckets = append(buckets, domainusage.NewBucket(
			query.Subject(),
			query.MeterName(),
			query.BucketSize(),
			bucketStart,
			row.Quantity,
		))
	}

	return buckets, nil
}

func (r *UsageRepository) queryDynamicBuckets(ctx context.Context, query domainusage.Query) ([]domainusage.Bucket, error) {
	args := []any{}
	groupBy := query.GroupByFields()
	groupSelectSQL, groupAliases, err := groupValueSelectSQL(groupBy, &args)
	if err != nil {
		return nil, err
	}
	partitionColumns := groupPartitionColumns(groupAliases)
	selectColumns := groupResultColumns(groupAliases)

	sqlQuery := strings.Builder{}
	sqlQuery.WriteString(`
WITH filtered AS (
	SELECT
		id,
		` + bucketStartSQL(query.BucketSize()) + ` AS bucket_start,
` + groupSelectSQL + `
		quantity,
		event_time AS event_at
	FROM usage_events
	WHERE meter_name = ?
		AND event_time >= ?
		AND event_time < ?
`)
	args = append(args, query.MeterName(), formatTime(query.From()), formatTime(query.To()))
	if query.Subject() != "" {
		sqlQuery.WriteString("\t\tAND subject = ?\n")
		args = append(args, query.Subject())
	}
	for key, value := range query.Metadata() {
		fieldSQL, err := metadataTextSQL(key, &args)
		if err != nil {
			return nil, err
		}
		sqlQuery.WriteString("\t\tAND " + fieldSQL + " = ?\n")
		args = append(args, value)
	}
	filterSQL, filterArgs, err := filterWhereSQL(query.Filter())
	if err != nil {
		return nil, err
	}
	if filterSQL != "" {
		sqlQuery.WriteString("\t\tAND ")
		sqlQuery.WriteString(filterSQL)
		sqlQuery.WriteString("\n")
		args = append(args, filterArgs...)
	}
	sqlQuery.WriteString(`
),
ranked AS (
	SELECT
		bucket_start,
` + groupColumnSelectSQL(groupAliases) + `
		quantity,
		ROW_NUMBER() OVER (PARTITION BY ` + partitionColumns + ` ORDER BY event_at ASC, id ASC) AS first_rank,
		ROW_NUMBER() OVER (PARTITION BY ` + partitionColumns + ` ORDER BY event_at DESC, id DESC) AS last_rank
	FROM filtered
)
SELECT ` + selectColumns + `, ` + aggregateSQL(query.Aggregation(), query.BucketSize()) + ` AS quantity
FROM ranked
GROUP BY ` + partitionColumns + `
ORDER BY ` + groupOrderColumns(groupAliases) + `
LIMIT ?
`)
	args = append(args, query.Limit())

	rows, err := r.store.QueryContext(ctx, sqlQuery.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	buckets := []domainusage.Bucket{}
	for rows.Next() {
		var bucketStartText string
		groupValues := make([]string, len(groupBy))
		var quantity float64
		scanTargets := []any{&bucketStartText}
		for i := range groupValues {
			scanTargets = append(scanTargets, &groupValues[i])
		}
		scanTargets = append(scanTargets, &quantity)
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, err
		}
		bucketStart, err := time.Parse(time.RFC3339Nano, bucketStartText)
		if err != nil {
			return nil, err
		}
		group := map[string]string{}
		bucketSubject := query.Subject()
		for index, field := range groupBy {
			group[field] = groupValues[index]
			if domainusage.IsSubjectGroupBy(field) {
				bucketSubject = groupValues[index]
			}
		}
		buckets = append(buckets, domainusage.NewBucketWithGroup(
			bucketSubject,
			query.MeterName(),
			query.BucketSize(),
			bucketStart,
			quantity,
			group,
		))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return buckets, nil
}

func (r *UsageRepository) FindDimensionValues(ctx context.Context, query domainusage.DimensionValueQuery) ([]domainusage.DimensionValue, error) {
	path, err := sqliteJSONPath(query.Field())
	if err != nil {
		return nil, err
	}

	rows, err := queriesFor(ctx, r.queries).ListUsageDimensionValues(ctx, sqlitedb.ListUsageDimensionValuesParams{
		Path:      path,
		MeterName: query.MeterName(),
		Subject:   eventStringValue(query.Subject()),
		FromTime:  eventTimeValue(query.From()),
		ToTime:    eventTimeValue(query.To()),
		Limit:     int64(query.Limit()),
	})
	if err != nil {
		return nil, err
	}

	values := make([]domainusage.DimensionValue, 0, len(rows))
	for _, row := range rows {
		values = append(values, domainusage.NewDimensionValue(query.Field(), row.Value, int(row.UsageEvents)))
	}

	return values, nil
}

func (r *UsageRepository) FindBreakdown(ctx context.Context, query domainusage.BreakdownQuery) ([]domainusage.BreakdownItem, error) {
	if query.Filter().IsZero() {
		return r.findBreakdown(ctx, query)
	}

	return r.findFilteredBreakdown(ctx, query)
}

func (r *UsageRepository) findBreakdown(ctx context.Context, query domainusage.BreakdownQuery) ([]domainusage.BreakdownItem, error) {
	path, err := sqliteJSONPath(query.Field())
	if err != nil {
		return nil, err
	}

	rows, err := queriesFor(ctx, r.queries).ListUsageBreakdown(ctx, sqlitedb.ListUsageBreakdownParams{
		Aggregation:     string(query.Aggregation()),
		DurationSeconds: query.To().Sub(query.From()).Seconds(),
		Limit:           int64(query.Limit()),
		Field:           query.Field(),
		Path:            path,
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

func (r *UsageRepository) findFilteredBreakdown(ctx context.Context, query domainusage.BreakdownQuery) ([]domainusage.BreakdownItem, error) {
	args := []any{}
	valueSQL, err := breakdownValueSQL(query.Field(), &args)
	if err != nil {
		return nil, err
	}

	sqlQuery := strings.Builder{}
	sqlQuery.WriteString(`
WITH filtered AS (
	SELECT
		id,
		` + valueSQL + ` AS value,
		quantity,
		event_time AS event_at
	FROM usage_events
	WHERE meter_name = ?
		AND event_time >= ?
		AND event_time < ?
`)
	args = append(args, query.MeterName(), formatTime(query.From()), formatTime(query.To()))
	if query.Subject() != "" {
		sqlQuery.WriteString("\t\tAND subject = ?\n")
		args = append(args, query.Subject())
	}
	filterSQL, filterArgs, err := filterWhereSQL(query.Filter())
	if err != nil {
		return nil, err
	}
	if filterSQL != "" {
		sqlQuery.WriteString("\t\tAND ")
		sqlQuery.WriteString(filterSQL)
		sqlQuery.WriteString("\n")
		args = append(args, filterArgs...)
	}
	sqlQuery.WriteString(`
),
ranked AS (
	SELECT
		value,
		quantity,
		ROW_NUMBER() OVER (PARTITION BY value ORDER BY event_at ASC, id ASC) AS first_rank,
		ROW_NUMBER() OVER (PARTITION BY value ORDER BY event_at DESC, id DESC) AS last_rank
	FROM filtered
	WHERE value IS NOT NULL AND value != ''
)
SELECT
	value,
	` + breakdownAggregateSQL(query.Aggregation(), &args, query.To().Sub(query.From()).Seconds()) + ` AS quantity,
	CAST(COUNT(*) AS INTEGER) AS usage_events
FROM ranked
GROUP BY value
ORDER BY quantity DESC, value ASC
LIMIT ?
`)
	args = append(args, query.Limit())

	rows, err := r.store.QueryContext(ctx, sqlQuery.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domainusage.BreakdownItem{}
	for rows.Next() {
		var value string
		var quantity float64
		var usageEvents int
		if err := rows.Scan(&value, &quantity, &usageEvents); err != nil {
			return nil, err
		}
		items = append(items, domainusage.NewBreakdownItem(query.Field(), value, quantity, usageEvents))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func bucketStartSQL(size domainusage.BucketSize) string {
	switch size {
	case domainusage.BucketHour:
		return "substr(event_time, 1, 13) || ':00:00Z'"
	case domainusage.BucketMonth:
		return "substr(event_time, 1, 7) || '-01T00:00:00Z'"
	default:
		return "substr(event_time, 1, 10) || 'T00:00:00Z'"
	}
}

func groupValueSelectSQL(groupBy []string, args *[]any) (string, []string, error) {
	columns := strings.Builder{}
	aliases := make([]string, 0, len(groupBy))
	for index, field := range groupBy {
		valueSQL := "subject"
		if !domainusage.IsSubjectGroupBy(field) {
			var err error
			valueSQL, err = metadataTextSQL(field, args)
			if err != nil {
				return "", nil, err
			}
		}
		alias := fmt.Sprintf("group_%d", index)
		columns.WriteString("\t\t")
		columns.WriteString(valueSQL)
		columns.WriteString(" AS ")
		columns.WriteString(alias)
		columns.WriteString(",\n")
		aliases = append(aliases, alias)
	}
	return columns.String(), aliases, nil
}

func groupColumnSelectSQL(aliases []string) string {
	if len(aliases) == 0 {
		return ""
	}

	return "\t\t" + strings.Join(aliases, ",\n\t\t") + ","
}

func groupPartitionColumns(aliases []string) string {
	columns := append([]string{"bucket_start"}, aliases...)
	return strings.Join(columns, ", ")
}

func groupResultColumns(aliases []string) string {
	return groupPartitionColumns(aliases)
}

func groupOrderColumns(aliases []string) string {
	columns := []string{"bucket_start ASC"}
	for _, alias := range aliases {
		columns = append(columns, alias+" ASC")
	}
	return strings.Join(columns, ", ")
}

func metadataTextSQL(key string, args *[]any) (string, error) {
	path, err := sqliteJSONPath(key)
	if err != nil {
		return "", err
	}
	*args = append(*args, path, path)
	return "CASE json_type(metadata, ?) WHEN 'true' THEN 'true' WHEN 'false' THEN 'false' ELSE COALESCE(CAST(json_extract(metadata, ?) AS TEXT), '<nil>') END", nil
}

func metadataValueSQL(key string, args *[]any) (string, error) {
	path, err := sqliteJSONPath(key)
	if err != nil {
		return "", err
	}
	*args = append(*args, path, path)
	return "CASE json_type(metadata, ?) WHEN 'true' THEN 'true' WHEN 'false' THEN 'false' ELSE CAST(json_extract(metadata, ?) AS TEXT) END", nil
}

func breakdownValueSQL(field string, args *[]any) (string, error) {
	if domainusage.IsSubjectGroupBy(field) {
		return "subject", nil
	}
	return metadataValueSQL(field, args)
}

func sqliteJSONPath(key string) (string, error) {
	if !metadataKeyPattern.MatchString(key) {
		return "", fmt.Errorf("unsupported metadata key %q", key)
	}
	return "$." + key, nil
}

func aggregateSQL(aggregation domainmeter.Aggregation, bucketSize domainusage.BucketSize) string {
	switch aggregation {
	case domainmeter.AggregationCount:
		return "CAST(COUNT(*) AS REAL)"
	case domainmeter.AggregationAverage:
		return "AVG(quantity)"
	case domainmeter.AggregationMinimum:
		return "MIN(quantity)"
	case domainmeter.AggregationMaximum:
		return "MAX(quantity)"
	case domainmeter.AggregationFirst:
		return "MAX(CASE WHEN first_rank = 1 THEN quantity END)"
	case domainmeter.AggregationLast:
		return "MAX(CASE WHEN last_rank = 1 THEN quantity END)"
	case domainmeter.AggregationRate:
		return "CAST(COUNT(*) AS REAL) / " + bucketDurationSecondsSQL(bucketSize)
	default:
		return "SUM(quantity)"
	}
}

func breakdownAggregateSQL(aggregation domainmeter.Aggregation, args *[]any, durationSeconds float64) string {
	switch aggregation {
	case domainmeter.AggregationCount:
		return "CAST(COUNT(*) AS REAL)"
	case domainmeter.AggregationAverage:
		return "AVG(quantity)"
	case domainmeter.AggregationMinimum:
		return "MIN(quantity)"
	case domainmeter.AggregationMaximum:
		return "MAX(quantity)"
	case domainmeter.AggregationFirst:
		return "MAX(CASE WHEN first_rank = 1 THEN quantity END)"
	case domainmeter.AggregationLast:
		return "MAX(CASE WHEN last_rank = 1 THEN quantity END)"
	case domainmeter.AggregationRate:
		*args = append(*args, durationSeconds)
		return "CAST(COUNT(*) AS REAL) / ?"
	default:
		return "SUM(quantity)"
	}
}

func bucketDurationSecondsSQL(size domainusage.BucketSize) string {
	switch size {
	case domainusage.BucketHour:
		return "3600.0"
	case domainusage.BucketMonth:
		return "CAST(strftime('%s', datetime(bucket_start, '+1 month')) - strftime('%s', bucket_start) AS REAL)"
	default:
		return "86400.0"
	}
}

func (r *UsageRepository) FindEvents(ctx context.Context, query domainusage.EventQuery) (domainusage.EventPage, error) {
	if query.Filter().IsZero() {
		return r.findEvents(ctx, query)
	}

	sqlQuery := strings.Builder{}
	sqlQuery.WriteString(`
SELECT id, idempotency_key, subject, meter_name, quantity, event_time, received_at, metadata
FROM usage_events
WHERE 1 = 1
`)
	args := []any{}
	if query.Subject() != "" {
		sqlQuery.WriteString(" AND subject = ?\n")
		args = append(args, query.Subject())
	}
	if query.MeterName() != "" {
		sqlQuery.WriteString(" AND meter_name = ?\n")
		args = append(args, query.MeterName())
	}
	if !query.From().IsZero() {
		sqlQuery.WriteString(" AND event_time >= ?\n")
		args = append(args, formatTime(query.From()))
	}
	if !query.To().IsZero() {
		sqlQuery.WriteString(" AND event_time < ?\n")
		args = append(args, formatTime(query.To()))
	}
	filterSQL, filterArgs, err := filterWhereSQL(query.Filter())
	if err != nil {
		return domainusage.EventPage{}, err
	}
	if filterSQL != "" {
		sqlQuery.WriteString(" AND ")
		sqlQuery.WriteString(filterSQL)
		sqlQuery.WriteString("\n")
		args = append(args, filterArgs...)
	}
	if !query.Cursor().IsZero() {
		sqlQuery.WriteString(" AND (event_time < ? OR (event_time = ? AND id < ?))\n")
		cursorTime := formatTime(query.Cursor().EventTime())
		args = append(args, cursorTime, cursorTime, query.Cursor().ID())
	}
	sqlQuery.WriteString("ORDER BY event_time DESC, id DESC\nLIMIT ?")
	args = append(args, query.Limit()+1)

	rows, err := r.store.QueryContext(ctx, sqlQuery.String(), args...)
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

func (r *UsageRepository) findEvents(ctx context.Context, query domainusage.EventQuery) (domainusage.EventPage, error) {
	cursorEventTime, cursorID := eventCursorValues(query.Cursor())
	rows, err := queriesFor(ctx, r.queries).ListUsageEvents(ctx, sqlitedb.ListUsageEventsParams{
		Subject:         eventStringValue(query.Subject()),
		MeterName:       eventStringValue(query.MeterName()),
		FromTime:        eventTimeValue(query.From()),
		ToTime:          eventTimeValue(query.To()),
		CursorEventTime: cursorEventTime,
		CursorID:        cursorID,
		Limit:           int64(query.Limit() + 1),
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

func filterWhereSQL(filter domainusage.Filter) (string, []any, error) {
	if filter.IsZero() {
		return "", nil, nil
	}

	switch filter.Type() {
	case domainusage.FilterTypeGroup:
		parts := []string{}
		args := []any{}
		for _, rule := range filter.Rules() {
			part, ruleArgs, err := filterWhereSQL(rule)
			if err != nil {
				return "", nil, err
			}
			if part == "" {
				continue
			}
			parts = append(parts, "("+part+")")
			args = append(args, ruleArgs...)
		}
		if len(parts) == 0 {
			return "", nil, nil
		}
		joiner := " AND "
		if filter.GroupOp() == domainusage.FilterGroupOr {
			joiner = " OR "
		}
		return strings.Join(parts, joiner), args, nil
	case domainusage.FilterTypeCondition:
		return conditionWhereSQL(filter)
	default:
		return "", nil, nil
	}
}

func conditionWhereSQL(filter domainusage.Filter) (string, []any, error) {
	fieldSQL, fieldArgs, valueKind, err := filterFieldSQL(filter.Field())
	if err != nil {
		return "", nil, err
	}

	op := filter.ConditionOp()
	if op == domainusage.FilterOpExists {
		if strings.HasPrefix(filter.Field(), "metadata.") {
			return "json_type(metadata, ?) IS NOT NULL", fieldArgs, nil
		}
		return fieldSQL + " IS NOT NULL", fieldArgs, nil
	}

	switch op {
	case domainusage.FilterOpEqual, domainusage.FilterOpNotEqual, domainusage.FilterOpGreaterThan, domainusage.FilterOpGreaterThanOrEqual, domainusage.FilterOpLessThan, domainusage.FilterOpLessThanOrEqual:
		value, err := sqlFilterValue(filter.Value(), valueKind)
		if err != nil {
			return "", nil, err
		}
		return fieldSQL + " " + sqlOperator(op) + " ?", append(fieldArgs, value), nil
	case domainusage.FilterOpIn:
		values, ok := filter.Value().([]any)
		if !ok || len(values) == 0 {
			return "", nil, fmt.Errorf("invalid in filter value")
		}
		placeholders := make([]string, 0, len(values))
		args := append([]any{}, fieldArgs...)
		for _, raw := range values {
			value, err := sqlFilterValue(raw, valueKind)
			if err != nil {
				return "", nil, err
			}
			placeholders = append(placeholders, "?")
			args = append(args, value)
		}
		return fieldSQL + " IN (" + strings.Join(placeholders, ", ") + ")", args, nil
	case domainusage.FilterOpContains:
		value, err := sqlFilterValue(filter.Value(), "text")
		if err != nil {
			return "", nil, err
		}
		return "CAST(" + fieldSQL + " AS TEXT) LIKE ?", append(fieldArgs, "%"+fmt.Sprint(value)+"%"), nil
	default:
		return "", nil, fmt.Errorf("unsupported filter operator %q", op)
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

func filterFieldSQL(field string) (string, []any, string, error) {
	switch field {
	case "subject":
		return "subject", nil, "text", nil
	case "meter":
		return "meter_name", nil, "text", nil
	case "quantity":
		return "quantity", nil, "number", nil
	case "timestamp", "event_time":
		return "event_time", nil, "time", nil
	case "received_at":
		return "received_at", nil, "time", nil
	case "idempotency_key":
		return "idempotency_key", nil, "text", nil
	default:
		key := strings.TrimPrefix(field, "metadata.")
		if key == field || !metadataKeyPattern.MatchString(key) {
			return "", nil, "", fmt.Errorf("unsupported filter field %q", field)
		}
		return "json_extract(metadata, ?)", []any{"$." + key}, "any", nil
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
	count, err := queriesFor(ctx, r.queries).CountUsageEvents(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *UsageRepository) FindMeterStats(ctx context.Context) ([]domainusage.MeterStats, error) {
	rows, err := queriesFor(ctx, r.queries).ListUsageMeterStats(ctx)
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
	cursorLastEventAt, cursorSubject := subjectStatsCursorValues(query)
	rows, err := queriesFor(ctx, r.queries).ListUsageSubjectStats(ctx, sqlitedb.ListUsageSubjectStatsParams{
		CursorLastEventAt: cursorLastEventAt,
		CursorSubject:     cursorSubject,
		Limit:             int64(query.Limit()),
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

func (r *UsageRepository) PruneEvents(ctx context.Context, query domainusage.PruneQuery) (int, error) {
	deleted, err := queriesFor(ctx, r.queries).PruneUsageEvents(ctx, sqlitedb.PruneUsageEventsParams{
		MeterName: query.MeterName(),
		EventTime: formatTime(query.Before()),
	})
	if err != nil {
		return 0, err
	}
	return int(deleted), nil
}

func (r *UsageRepository) CountPrunableEvents(ctx context.Context, query domainusage.PruneQuery) (int, error) {
	count, err := queriesFor(ctx, r.queries).CountPrunableUsageEvents(ctx, sqlitedb.CountPrunableUsageEventsParams{
		MeterName: query.MeterName(),
		EventTime: formatTime(query.Before()),
	})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *UsageRepository) SavePruneRun(ctx context.Context, run domainusage.PruneRun) (domainusage.PruneRun, error) {
	meters, err := marshalPruneRunMeters(run.Meters())
	if err != nil {
		return domainusage.PruneRun{}, err
	}

	err = queriesFor(ctx, r.queries).SaveUsagePruneRun(ctx, sqlitedb.SaveUsagePruneRunParams{
		ID:        run.ID(),
		DryRun:    int64(boolInt(run.DryRun())),
		Deleted:   int64(run.Deleted()),
		Meters:    meters,
		CreatedAt: formatTime(run.CreatedAt()),
	})
	if err != nil {
		return domainusage.PruneRun{}, err
	}

	return run, nil
}

func (r *UsageRepository) FindPruneRuns(ctx context.Context, query domainusage.RunQuery) ([]domainusage.PruneRun, error) {
	cursorCreatedAt, cursorID := runCursorValues(query)
	rows, err := queriesFor(ctx, r.queries).ListUsagePruneRuns(ctx, sqlitedb.ListUsagePruneRunsParams{
		CursorCreatedAt: cursorCreatedAt,
		CursorID:        cursorID,
		Limit:           int64(query.Limit()),
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
	count, err := queriesFor(ctx, r.queries).CountUsagePruneRuns(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *UsageRepository) SaveIngestionRun(ctx context.Context, run domainusage.IngestionRun) (domainusage.IngestionRun, error) {
	err := queriesFor(ctx, r.queries).SaveUsageIngestionRun(ctx, sqlitedb.SaveUsageIngestionRunParams{
		ID:         run.ID(),
		Kind:       string(run.Kind()),
		Accepted:   int64(run.Accepted()),
		Duplicates: int64(run.Duplicates()),
		Failed:     int64(run.Failed()),
		CreatedAt:  formatTime(run.CreatedAt()),
	})
	if err != nil {
		return domainusage.IngestionRun{}, err
	}

	return run, nil
}

func (r *UsageRepository) FindIngestionRuns(ctx context.Context, query domainusage.RunQuery) ([]domainusage.IngestionRun, error) {
	cursorCreatedAt, cursorID := runCursorValues(query)
	rows, err := queriesFor(ctx, r.queries).ListUsageIngestionRuns(ctx, sqlitedb.ListUsageIngestionRunsParams{
		CursorCreatedAt: cursorCreatedAt,
		CursorID:        cursorID,
		Limit:           int64(query.Limit()),
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

func (r *UsageRepository) findByIdempotencyKey(ctx context.Context, key string) (domainusage.Event, error) {
	event, err := queriesFor(ctx, r.queries).FindUsageEventByIdempotencyKey(ctx, sql.NullString{String: key, Valid: true})
	return eventFromFields(event.ID, event.IdempotencyKey, event.Subject, event.MeterName, event.Quantity, event.EventTime, event.ReceivedAt, event.Metadata, err)
}

func (r *UsageRepository) findByID(ctx context.Context, id string) (domainusage.Event, error) {
	event, err := queriesFor(ctx, r.queries).FindUsageEventByID(ctx, id)
	return eventFromFields(event.ID, event.IdempotencyKey, event.Subject, event.MeterName, event.Quantity, event.EventTime, event.ReceivedAt, event.Metadata, err)
}

func (r *UsageRepository) findBulk(ctx context.Context, idempotencyKey string) (domainusage.BulkSaveResult, error) {
	response, err := queriesFor(ctx, r.queries).FindBulkUsageIngestion(ctx, idempotencyKey)
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

func ingestionRunFromFields(id string, kind string, accepted int64, duplicates int64, failed int64, createdAtText string) (domainusage.IngestionRun, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtText)
	if err != nil {
		return domainusage.IngestionRun{}, err
	}

	return domainusage.NewIngestionRun(id, domainusage.IngestionKind(kind), int(accepted), int(duplicates), int(failed), createdAt)
}

func eventFromFields(id string, idempotencyKey sql.NullString, subject string, meterName string, quantity float64, eventTimeText string, receivedAtText string, metadataText string, err error) (domainusage.Event, error) {
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

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func pruneRunFromFields(id string, dryRun int64, deleted int64, metersText string, createdAtText string) (domainusage.PruneRun, error) {
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

	return eventFromFields(id, idempotencyKey, subject, meterName, quantity, eventTimeText, receivedAtText, metadataText, nil)
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
