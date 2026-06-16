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

	"github.com/ssubedir/open-spanner/internal/metering/adapters/postgres/postgresdb"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

var metadataKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_]+(\.[A-Za-z0-9_]+)*$`)

var errBulkReplay = errors.New("bulk ingestion already exists")

const (
	pruneAdvisoryLockKey = int64(0x4f535052554e45)
	pruneDeleteBatchSize = 1000
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

		err = r.queries.SaveBulkUsageIngestion(txCtx, postgresdb.SaveBulkUsageIngestionParams{
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

	err = r.queries.SaveUsageEvent(ctx, postgresdb.SaveUsageEventParams{
		ID:             event.ID(),
		IdempotencyKey: event.IdempotencyKey(),
		Subject:        event.Subject(),
		MeterName:      event.MeterName(),
		Quantity:       event.Quantity(),
		EventTime:      formatTime(event.EventTime()),
		ReceivedAt:     formatTime(event.ReceivedAt()),
		Metadata:       json.RawMessage(metadata),
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
	groupBy := query.GroupByFields()
	groupSelectSQL, groupAliases, err := groupValueSelectSQL(groupBy)
	if err != nil {
		return nil, err
	}
	partitionColumns := groupPartitionColumns(groupAliases)
	selectColumns := groupResultColumns(groupAliases)

	sqlQuery := strings.Builder{}
	sqlQuery.WriteString(`
WITH filtered AS (
	SELECT
		` + bucketStartSQL(query.BucketSize()) + ` AS bucket_start,
` + groupSelectSQL + `
		quantity,
		event_time::timestamptz AS event_at
	FROM usage_events
	WHERE meter_name = `)
	args := []any{}
	sqlQuery.WriteString(bindArg(&args, query.MeterName()))
	sqlQuery.WriteString("\n")
	sqlQuery.WriteString("\t\tAND event_time >= " + bindArg(&args, formatTime(query.From())) + "\n")
	sqlQuery.WriteString("\t\tAND event_time < " + bindArg(&args, formatTime(query.To())) + "\n")
	if query.Subject() != "" {
		sqlQuery.WriteString("\t\tAND subject = " + bindArg(&args, query.Subject()) + "\n")
	}
	for key, value := range query.Metadata() {
		if !metadataKeyPattern.MatchString(key) {
			return nil, fmt.Errorf("unsupported metadata filter key %q", key)
		}
		sqlQuery.WriteString("\t\tAND metadata #>> " + postgresJSONPath(key) + " = " + bindArg(&args, value) + "\n")
	}
	filterSQL, err := filterWhereSQL(query.Filter(), &args)
	if err != nil {
		return nil, err
	}
	if filterSQL != "" {
		sqlQuery.WriteString("\t\tAND ")
		sqlQuery.WriteString(filterSQL)
		sqlQuery.WriteString("\n")
	}
	sqlQuery.WriteString(`)
SELECT ` + selectColumns + `, ` + aggregateSQL(query.Aggregation(), query.BucketSize()) + ` AS quantity
FROM filtered
GROUP BY ` + partitionColumns + `
ORDER BY ` + groupOrderColumns(groupAliases) + `
LIMIT ` + bindArg(&args, query.Limit()) + `
`)

	rows, err := r.store.query(ctx, sqlQuery.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	buckets := []domainusage.Bucket{}
	for rows.Next() {
		var bucketStart time.Time
		groupValues := make([]string, len(groupBy))
		var quantity float64
		scanTargets := []any{&bucketStart}
		for i := range groupValues {
			scanTargets = append(scanTargets, &groupValues[i])
		}
		scanTargets = append(scanTargets, &quantity)
		if err := rows.Scan(scanTargets...); err != nil {
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
	if !metadataKeyPattern.MatchString(query.Field()) {
		return nil, fmt.Errorf("unsupported metadata field %q", query.Field())
	}

	args := []any{}
	valueSQL := "metadata #>> " + postgresJSONPath(query.Field())
	sqlQuery := strings.Builder{}
	sqlQuery.WriteString("SELECT value, COUNT(*) AS usage_events\n")
	sqlQuery.WriteString("FROM (\n")
	sqlQuery.WriteString("\tSELECT " + valueSQL + " AS value\n")
	sqlQuery.WriteString("\tFROM usage_events\n")
	sqlQuery.WriteString("\tWHERE meter_name = " + bindArg(&args, query.MeterName()) + "\n")
	sqlQuery.WriteString("\t\tAND " + valueSQL + " IS NOT NULL\n")
	if query.Subject() != "" {
		sqlQuery.WriteString("\t\tAND subject = " + bindArg(&args, query.Subject()) + "\n")
	}
	if !query.From().IsZero() {
		sqlQuery.WriteString("\t\tAND event_time >= " + bindArg(&args, formatTime(query.From())) + "\n")
	}
	if !query.To().IsZero() {
		sqlQuery.WriteString("\t\tAND event_time < " + bindArg(&args, formatTime(query.To())) + "\n")
	}
	sqlQuery.WriteString(") discovered\n")
	sqlQuery.WriteString("WHERE value IS NOT NULL AND value != ''\n")
	sqlQuery.WriteString("GROUP BY value\n")
	sqlQuery.WriteString("ORDER BY usage_events DESC, value ASC\n")
	sqlQuery.WriteString("LIMIT " + bindArg(&args, query.Limit()))

	rows, err := r.store.query(ctx, sqlQuery.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := []domainusage.DimensionValue{}
	for rows.Next() {
		var value string
		var usageEvents int
		if err := rows.Scan(&value, &usageEvents); err != nil {
			return nil, err
		}
		values = append(values, domainusage.NewDimensionValue(query.Field(), value, usageEvents))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return values, nil
}

func (r *UsageRepository) FindBreakdown(ctx context.Context, query domainusage.BreakdownQuery) ([]domainusage.BreakdownItem, error) {
	valueSQL, err := breakdownValueSQL(query.Field())
	if err != nil {
		return nil, err
	}

	args := []any{}
	sqlQuery := strings.Builder{}
	sqlQuery.WriteString(`
WITH filtered AS (
	SELECT
		` + valueSQL + ` AS value,
		quantity,
		event_time::timestamptz AS event_at
	FROM usage_events
	WHERE meter_name = `)
	sqlQuery.WriteString(bindArg(&args, query.MeterName()))
	sqlQuery.WriteString("\n")
	sqlQuery.WriteString("\t\tAND event_time >= " + bindArg(&args, formatTime(query.From())) + "\n")
	sqlQuery.WriteString("\t\tAND event_time < " + bindArg(&args, formatTime(query.To())) + "\n")
	if query.Subject() != "" {
		sqlQuery.WriteString("\t\tAND subject = " + bindArg(&args, query.Subject()) + "\n")
	}
	filterSQL, err := filterWhereSQL(query.Filter(), &args)
	if err != nil {
		return nil, err
	}
	if filterSQL != "" {
		sqlQuery.WriteString("\t\tAND ")
		sqlQuery.WriteString(filterSQL)
		sqlQuery.WriteString("\n")
	}
	sqlQuery.WriteString(`)
SELECT
	value,
	` + breakdownAggregateSQL(query.Aggregation(), &args, query.To().Sub(query.From()).Seconds()) + ` AS quantity,
	COUNT(*) AS usage_events
FROM filtered
WHERE value IS NOT NULL AND value != ''
GROUP BY value
ORDER BY quantity DESC, value ASC
LIMIT ` + bindArg(&args, query.Limit()) + `
`)

	rows, err := r.store.query(ctx, sqlQuery.String(), args...)
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
		return "date_trunc('hour', event_time::timestamptz)"
	case domainusage.BucketMonth:
		return "date_trunc('month', event_time::timestamptz)"
	default:
		return "date_trunc('day', event_time::timestamptz)"
	}
}

func groupValueSelectSQL(groupBy []string) (string, []string, error) {
	columns := strings.Builder{}
	aliases := make([]string, 0, len(groupBy))
	for index, field := range groupBy {
		alias := fmt.Sprintf("group_%d", index)
		if domainusage.IsSubjectGroupBy(field) {
			columns.WriteString("\t\tsubject AS ")
		} else {
			if !metadataKeyPattern.MatchString(field) {
				return "", nil, fmt.Errorf("unsupported group by field %q", field)
			}
			columns.WriteString("\t\tCOALESCE(metadata #>> ")
			columns.WriteString(postgresJSONPath(field))
			columns.WriteString(", '<nil>') AS ")
		}
		columns.WriteString(alias)
		columns.WriteString(",\n")
		aliases = append(aliases, alias)
	}
	return columns.String(), aliases, nil
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

func aggregateSQL(aggregation domainmeter.Aggregation, bucketSize domainusage.BucketSize) string {
	switch aggregation {
	case domainmeter.AggregationCount:
		return "COUNT(*)::double precision"
	case domainmeter.AggregationAverage:
		return "AVG(quantity)"
	case domainmeter.AggregationMinimum:
		return "MIN(quantity)"
	case domainmeter.AggregationMaximum:
		return "MAX(quantity)"
	case domainmeter.AggregationFirst:
		return "(array_agg(quantity ORDER BY event_at ASC))[1]"
	case domainmeter.AggregationLast:
		return "(array_agg(quantity ORDER BY event_at DESC))[1]"
	case domainmeter.AggregationRate:
		return "COUNT(*)::double precision / " + bucketDurationSecondsSQL(bucketSize)
	default:
		return "SUM(quantity)"
	}
}

func breakdownValueSQL(field string) (string, error) {
	if domainusage.IsSubjectGroupBy(field) {
		return "subject", nil
	}
	if !metadataKeyPattern.MatchString(field) {
		return "", fmt.Errorf("unsupported breakdown field %q", field)
	}
	return "metadata #>> " + postgresJSONPath(field), nil
}

func breakdownAggregateSQL(aggregation domainmeter.Aggregation, args *[]any, durationSeconds float64) string {
	switch aggregation {
	case domainmeter.AggregationCount:
		return "COUNT(*)::double precision"
	case domainmeter.AggregationAverage:
		return "AVG(quantity)"
	case domainmeter.AggregationMinimum:
		return "MIN(quantity)"
	case domainmeter.AggregationMaximum:
		return "MAX(quantity)"
	case domainmeter.AggregationFirst:
		return "(array_agg(quantity ORDER BY event_at ASC))[1]"
	case domainmeter.AggregationLast:
		return "(array_agg(quantity ORDER BY event_at DESC))[1]"
	case domainmeter.AggregationRate:
		return "COUNT(*)::double precision / " + bindArg(args, durationSeconds)
	default:
		return "SUM(quantity)"
	}
}

func bucketDurationSecondsSQL(size domainusage.BucketSize) string {
	switch size {
	case domainusage.BucketHour:
		return "3600::double precision"
	case domainusage.BucketMonth:
		return "EXTRACT(EPOCH FROM (bucket_start + INTERVAL '1 month' - bucket_start))"
	default:
		return "86400::double precision"
	}
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

type filterField struct {
	expression  string
	valueKind   string
	metadataKey string
}

func conditionWhereSQL(filter domainusage.Filter, args *[]any) (string, error) {
	field, err := filterFieldSQL(filter.Field())
	if err != nil {
		return "", err
	}

	op := filter.ConditionOp()
	if op == domainusage.FilterOpExists {
		return field.expression + " IS NOT NULL", nil
	}

	switch op {
	case domainusage.FilterOpEqual:
		if field.metadataKey != "" {
			return metadataContainsSQL(field.metadataKey, filter.Value(), args)
		}
		value, err := sqlFilterValue(filter.Value(), field.valueKind)
		if err != nil {
			return "", err
		}
		return field.expression + " = " + bindArg(args, value), nil
	case domainusage.FilterOpNotEqual, domainusage.FilterOpGreaterThan, domainusage.FilterOpGreaterThanOrEqual, domainusage.FilterOpLessThan, domainusage.FilterOpLessThanOrEqual:
		value, err := sqlFilterValue(filter.Value(), field.valueKind)
		if err != nil {
			return "", err
		}
		return field.expression + " " + sqlOperator(op) + " " + bindArg(args, value), nil
	case domainusage.FilterOpIn:
		values, ok := filter.Value().([]any)
		if !ok || len(values) == 0 {
			return "", fmt.Errorf("invalid in filter value")
		}
		if field.metadataKey != "" {
			parts := make([]string, 0, len(values))
			for _, raw := range values {
				part, err := metadataContainsSQL(field.metadataKey, raw, args)
				if err != nil {
					return "", err
				}
				parts = append(parts, part)
			}
			return "(" + strings.Join(parts, " OR ") + ")", nil
		}
		placeholders := make([]string, 0, len(values))
		for _, raw := range values {
			value, err := sqlFilterValue(raw, field.valueKind)
			if err != nil {
				return "", err
			}
			placeholders = append(placeholders, bindArg(args, value))
		}
		return field.expression + " IN (" + strings.Join(placeholders, ", ") + ")", nil
	case domainusage.FilterOpContains:
		value, err := sqlFilterValue(filter.Value(), "text")
		if err != nil {
			return "", err
		}
		return "CAST(" + field.expression + " AS TEXT) LIKE " + bindArg(args, "%"+fmt.Sprint(value)+"%"), nil
	default:
		return "", fmt.Errorf("unsupported filter operator %q", op)
	}
}

func metadataContainsSQL(key string, value any, args *[]any) (string, error) {
	payload, err := metadataContainsJSON(key, value)
	if err != nil {
		return "", err
	}
	return "metadata @> " + bindArg(args, payload) + "::jsonb", nil
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

func filterFieldSQL(field string) (filterField, error) {
	switch field {
	case "subject":
		return filterField{expression: "subject", valueKind: "text"}, nil
	case "meter":
		return filterField{expression: "meter_name", valueKind: "text"}, nil
	case "quantity":
		return filterField{expression: "quantity", valueKind: "number"}, nil
	case "timestamp", "event_time":
		return filterField{expression: "event_time", valueKind: "time"}, nil
	case "received_at":
		return filterField{expression: "received_at", valueKind: "time"}, nil
	case "idempotency_key":
		return filterField{expression: "idempotency_key", valueKind: "text"}, nil
	default:
		key := strings.TrimPrefix(field, "metadata.")
		if key == field || !metadataKeyPattern.MatchString(key) {
			return filterField{}, fmt.Errorf("unsupported filter field %q", field)
		}
		return filterField{expression: "metadata #>> " + postgresJSONPath(key), valueKind: "text", metadataKey: key}, nil
	}
}

func metadataContainsJSON(key string, value any) (string, error) {
	if !metadataKeyPattern.MatchString(key) {
		return "", fmt.Errorf("unsupported metadata filter key %q", key)
	}

	parts := strings.Split(key, ".")
	var node any = value
	for i := len(parts) - 1; i >= 0; i-- {
		node = map[string]any{parts[i]: node}
	}

	payload, err := json.Marshal(node)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func sqlFilterValue(value any, kind string) (any, error) {
	if kind == "text" {
		return fmt.Sprint(value), nil
	}
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
	return r.queries.TryPruneLock(ctx, pruneAdvisoryLockKey)
}

func (r *UsageRepository) PruneEvents(ctx context.Context, query domainusage.PruneQuery) (int, error) {
	total := 0
	for {
		deleted, err := r.queries.PruneUsageEventsBatch(ctx, postgresdb.PruneUsageEventsBatchParams{
			MeterName: query.MeterName(),
			EventTime: formatTime(query.Before()),
			Limit:     int32(pruneDeleteBatchSize),
		})
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
	count, err := r.queries.CountPrunableUsageEvents(ctx, postgresdb.CountPrunableUsageEventsParams{
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

	err = r.queries.SaveUsagePruneRun(ctx, postgresdb.SaveUsagePruneRunParams{
		ID:        run.ID(),
		DryRun:    int32(boolInt(run.DryRun())),
		Deleted:   int32(run.Deleted()),
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
	rows, err := r.queries.ListUsagePruneRuns(ctx, postgresdb.ListUsagePruneRunsParams{
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
	count, err := r.queries.CountUsagePruneRuns(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *UsageRepository) SaveIngestionRun(ctx context.Context, run domainusage.IngestionRun) (domainusage.IngestionRun, error) {
	err := r.queries.SaveUsageIngestionRun(ctx, postgresdb.SaveUsageIngestionRunParams{
		ID:         run.ID(),
		Kind:       string(run.Kind()),
		Accepted:   int32(run.Accepted()),
		Duplicates: int32(run.Duplicates()),
		Failed:     int32(run.Failed()),
		CreatedAt:  formatTime(run.CreatedAt()),
	})
	if err != nil {
		return domainusage.IngestionRun{}, err
	}

	return run, nil
}

func (r *UsageRepository) FindIngestionRuns(ctx context.Context, query domainusage.RunQuery) ([]domainusage.IngestionRun, error) {
	cursorCreatedAt, cursorID := runCursorValues(query)
	rows, err := r.queries.ListUsageIngestionRuns(ctx, postgresdb.ListUsageIngestionRunsParams{
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

func (r *UsageRepository) findByIdempotencyKey(ctx context.Context, key string) (domainusage.Event, error) {
	event, err := r.queries.FindUsageEventByIdempotencyKey(ctx, sql.NullString{String: key, Valid: true})
	return eventFromFields(event.ID, event.IdempotencyKey, event.Subject, event.MeterName, event.Quantity, event.EventTime, event.ReceivedAt, event.Metadata, err)
}

func (r *UsageRepository) findByID(ctx context.Context, id string) (domainusage.Event, error) {
	event, err := r.queries.FindUsageEventByID(ctx, id)
	return eventFromFields(event.ID, event.IdempotencyKey, event.Subject, event.MeterName, event.Quantity, event.EventTime, event.ReceivedAt, event.Metadata, err)
}

func postgresJSONPath(key string) string {
	return "'{" + strings.ReplaceAll(key, ".", ",") + "}'"
}

func (r *UsageRepository) findBulk(ctx context.Context, idempotencyKey string) (domainusage.BulkSaveResult, error) {
	response, err := r.queries.FindBulkUsageIngestion(ctx, idempotencyKey)
	if err != nil {
		return domainusage.BulkSaveResult{}, err
	}

	return unmarshalBulkResult(response)
}

func runCursorValues(query domainusage.RunQuery) (sql.NullString, sql.NullString) {
	if !query.HasCursor() {
		return sql.NullString{}, sql.NullString{}
	}
	return sql.NullString{String: formatTime(query.CreatedAt()), Valid: true}, sql.NullString{String: query.ID(), Valid: true}
}

func ingestionRunFromFields(id string, kind string, accepted int32, duplicates int32, failed int32, createdAtText string) (domainusage.IngestionRun, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtText)
	if err != nil {
		return domainusage.IngestionRun{}, err
	}

	return domainusage.NewIngestionRun(id, domainusage.IngestionKind(kind), int(accepted), int(duplicates), int(failed), createdAt)
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
