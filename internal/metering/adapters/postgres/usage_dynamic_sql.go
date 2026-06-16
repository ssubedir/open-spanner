package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

// This file owns the Postgres SQL that must be assembled at runtime. Fixed
// query shapes live in sqlc queries and are called from usage_repository.go.

var postgresUsageDialect = goqu.Dialect("postgres")

const bucketStartAlias = "bucket_start"

type sqlOperand interface {
	exp.Expression
	exp.Comparable
	exp.Inable
	exp.Isable
	exp.Likeable
}

func (r *UsageRepository) queryBucketsWithDynamicSQL(ctx context.Context, query domainusage.Query) ([]domainusage.Bucket, error) {
	groupBy := query.GroupByFields()
	groupSelects, groupAliases, err := groupSelectExpressions(groupBy)
	if err != nil {
		return nil, err
	}

	filteredSelects := []interface{}{
		bucketStartExpression(query.BucketSize()).As(bucketStartAlias),
	}
	filteredSelects = append(filteredSelects, groupSelects...)
	filteredSelects = append(filteredSelects,
		goqu.C("quantity"),
		goqu.L("event_time::timestamptz").As("event_at"),
	)

	predicates, err := bucketPredicates(query)
	if err != nil {
		return nil, err
	}

	filtered := postgresUsageDialect.
		From("usage_events").
		Prepared(true).
		Select(filteredSelects...).
		Where(predicates...)

	resultSelects := groupResultSelects(groupAliases)
	resultSelects = append(resultSelects, bucketAggregationExpression(query.Aggregation(), query.BucketSize()).As("quantity"))

	sqlQuery, args, err := postgresUsageDialect.
		From(filtered.As("filtered")).
		Prepared(true).
		Select(resultSelects...).
		GroupBy(groupExpressions(groupAliases)...).
		Order(groupOrderExpressions(groupAliases)...).
		Limit(uint(query.Limit())).
		ToSQL()
	if err != nil {
		return nil, err
	}

	rows, err := r.store.QueryContext(ctx, sqlQuery, args...)
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

func (r *UsageRepository) findBreakdownWithDynamicSQL(ctx context.Context, query domainusage.BreakdownQuery) ([]domainusage.BreakdownItem, error) {
	valueExpression, err := breakdownFieldExpression(query.Field())
	if err != nil {
		return nil, err
	}
	predicates, err := breakdownPredicates(query)
	if err != nil {
		return nil, err
	}

	filtered := postgresUsageDialect.
		From("usage_events").
		Prepared(true).
		Select(
			valueExpression.As("value"),
			goqu.C("quantity"),
			goqu.L("event_time::timestamptz").As("event_at"),
		).
		Where(predicates...)

	sqlQuery, args, err := postgresUsageDialect.
		From(filtered.As("filtered")).
		Prepared(true).
		Select(
			goqu.C("value"),
			breakdownAggregationExpression(query.Aggregation(), query.To().Sub(query.From()).Seconds()).As("quantity"),
			goqu.L("COUNT(*)").As("usage_events"),
		).
		Where(goqu.C("value").IsNotNull(), goqu.C("value").Neq("")).
		GroupBy(goqu.C("value")).
		Order(goqu.C("quantity").Desc(), goqu.C("value").Asc()).
		Limit(uint(query.Limit())).
		ToSQL()
	if err != nil {
		return nil, err
	}

	rows, err := r.store.QueryContext(ctx, sqlQuery, args...)
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

func (r *UsageRepository) findEventsWithDynamicSQL(ctx context.Context, query domainusage.EventQuery) (domainusage.EventPage, error) {
	predicates, err := eventPredicates(query)
	if err != nil {
		return domainusage.EventPage{}, err
	}

	dataset := postgresUsageDialect.
		From("usage_events").
		Prepared(true).
		Select("id", "idempotency_key", "subject", "meter_name", "quantity", "event_time", "received_at", "metadata").
		Order(goqu.C("event_time").Desc(), goqu.C("id").Desc()).
		Limit(uint(query.Limit() + 1))
	if len(predicates) > 0 {
		dataset = dataset.Where(predicates...)
	}

	sqlQuery, args, err := dataset.ToSQL()
	if err != nil {
		return domainusage.EventPage{}, err
	}

	rows, err := r.store.QueryContext(ctx, sqlQuery, args...)
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

func bucketPredicates(query domainusage.Query) ([]exp.Expression, error) {
	predicates := []exp.Expression{
		goqu.C("meter_name").Eq(query.MeterName()),
		goqu.C("event_time").Gte(formatTime(query.From())),
		goqu.C("event_time").Lt(formatTime(query.To())),
	}
	if query.Subject() != "" {
		predicates = append(predicates, goqu.C("subject").Eq(query.Subject()))
	}
	for key, value := range query.Metadata() {
		expression, err := metadataTextExpression(key)
		if err != nil {
			return nil, err
		}
		predicates = append(predicates, expression.Eq(value))
	}
	filterExpression, err := filterPredicateExpression(query.Filter())
	if err != nil {
		return nil, err
	}
	if filterExpression != nil {
		predicates = append(predicates, filterExpression)
	}
	return predicates, nil
}

func breakdownPredicates(query domainusage.BreakdownQuery) ([]exp.Expression, error) {
	predicates := []exp.Expression{
		goqu.C("meter_name").Eq(query.MeterName()),
		goqu.C("event_time").Gte(formatTime(query.From())),
		goqu.C("event_time").Lt(formatTime(query.To())),
	}
	if query.Subject() != "" {
		predicates = append(predicates, goqu.C("subject").Eq(query.Subject()))
	}
	filterExpression, err := filterPredicateExpression(query.Filter())
	if err != nil {
		return nil, err
	}
	if filterExpression != nil {
		predicates = append(predicates, filterExpression)
	}
	return predicates, nil
}

func eventPredicates(query domainusage.EventQuery) ([]exp.Expression, error) {
	predicates := []exp.Expression{}
	if query.Subject() != "" {
		predicates = append(predicates, goqu.C("subject").Eq(query.Subject()))
	}
	if query.MeterName() != "" {
		predicates = append(predicates, goqu.C("meter_name").Eq(query.MeterName()))
	}
	if !query.From().IsZero() {
		predicates = append(predicates, goqu.C("event_time").Gte(formatTime(query.From())))
	}
	if !query.To().IsZero() {
		predicates = append(predicates, goqu.C("event_time").Lt(formatTime(query.To())))
	}
	filterExpression, err := filterPredicateExpression(query.Filter())
	if err != nil {
		return nil, err
	}
	if filterExpression != nil {
		predicates = append(predicates, filterExpression)
	}
	if !query.Cursor().IsZero() {
		cursorTime := formatTime(query.Cursor().EventTime())
		predicates = append(predicates, goqu.Or(
			goqu.C("event_time").Lt(cursorTime),
			goqu.And(
				goqu.C("event_time").Eq(cursorTime),
				goqu.C("id").Lt(query.Cursor().ID()),
			),
		))
	}
	return predicates, nil
}

func bucketStartExpression(size domainusage.BucketSize) exp.LiteralExpression {
	switch size {
	case domainusage.BucketHour:
		return goqu.L("date_trunc('hour', event_time::timestamptz)")
	case domainusage.BucketMonth:
		return goqu.L("date_trunc('month', event_time::timestamptz)")
	default:
		return goqu.L("date_trunc('day', event_time::timestamptz)")
	}
}

func groupSelectExpressions(groupBy []string) ([]interface{}, []string, error) {
	selects := make([]interface{}, 0, len(groupBy))
	aliases := make([]string, 0, len(groupBy))
	for index, field := range groupBy {
		var value exp.Aliaseable
		if domainusage.IsSubjectGroupBy(field) {
			value = goqu.C("subject")
		} else {
			metadataValue, err := metadataTextExpression(field)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: unsupported group by field %q", domain.ErrInvalidInput, field)
			}
			value = goqu.L("COALESCE(?, '<nil>')", metadataValue)
		}
		alias := groupAlias(index)
		selects = append(selects, value.As(alias))
		aliases = append(aliases, alias)
	}
	return selects, aliases, nil
}

func groupResultSelects(aliases []string) []interface{} {
	selects := []interface{}{goqu.C(bucketStartAlias)}
	for _, alias := range aliases {
		selects = append(selects, goqu.C(alias))
	}
	return selects
}

func groupExpressions(aliases []string) []interface{} {
	expressions := []interface{}{goqu.C(bucketStartAlias)}
	for _, alias := range aliases {
		expressions = append(expressions, goqu.C(alias))
	}
	return expressions
}

func groupOrderExpressions(aliases []string) []exp.OrderedExpression {
	expressions := []exp.OrderedExpression{goqu.C(bucketStartAlias).Asc()}
	for _, alias := range aliases {
		expressions = append(expressions, goqu.C(alias).Asc())
	}
	return expressions
}

func groupAlias(index int) string {
	return fmt.Sprintf("group_%d", index)
}

func bucketAggregationExpression(aggregation domainmeter.Aggregation, bucketSize domainusage.BucketSize) exp.LiteralExpression {
	switch aggregation {
	case domainmeter.AggregationCount:
		return goqu.L("COUNT(*)::double precision")
	case domainmeter.AggregationAverage:
		return goqu.L("AVG(quantity)")
	case domainmeter.AggregationMinimum:
		return goqu.L("MIN(quantity)")
	case domainmeter.AggregationMaximum:
		return goqu.L("MAX(quantity)")
	case domainmeter.AggregationFirst:
		return goqu.L("(array_agg(quantity ORDER BY event_at ASC))[1]")
	case domainmeter.AggregationLast:
		return goqu.L("(array_agg(quantity ORDER BY event_at DESC))[1]")
	case domainmeter.AggregationRate:
		return goqu.L("COUNT(*)::double precision / ?", bucketDurationSecondsExpression(bucketSize))
	default:
		return goqu.L("SUM(quantity)")
	}
}

func breakdownFieldExpression(field string) (exp.LiteralExpression, error) {
	if domainusage.IsSubjectGroupBy(field) {
		return goqu.L("subject"), nil
	}
	return metadataTextExpression(field)
}

func breakdownAggregationExpression(aggregation domainmeter.Aggregation, durationSeconds float64) exp.LiteralExpression {
	switch aggregation {
	case domainmeter.AggregationCount:
		return goqu.L("COUNT(*)::double precision")
	case domainmeter.AggregationAverage:
		return goqu.L("AVG(quantity)")
	case domainmeter.AggregationMinimum:
		return goqu.L("MIN(quantity)")
	case domainmeter.AggregationMaximum:
		return goqu.L("MAX(quantity)")
	case domainmeter.AggregationFirst:
		return goqu.L("(array_agg(quantity ORDER BY event_at ASC))[1]")
	case domainmeter.AggregationLast:
		return goqu.L("(array_agg(quantity ORDER BY event_at DESC))[1]")
	case domainmeter.AggregationRate:
		return goqu.L("COUNT(*)::double precision / ?", durationSeconds)
	default:
		return goqu.L("SUM(quantity)")
	}
}

func bucketDurationSecondsExpression(size domainusage.BucketSize) exp.LiteralExpression {
	switch size {
	case domainusage.BucketHour:
		return goqu.L("3600::double precision")
	case domainusage.BucketMonth:
		return goqu.L("EXTRACT(EPOCH FROM (? + INTERVAL '1 month' - ?))", goqu.C(bucketStartAlias), goqu.C(bucketStartAlias))
	default:
		return goqu.L("86400::double precision")
	}
}

func filterPredicateExpression(filter domainusage.Filter) (exp.Expression, error) {
	if filter.IsZero() {
		return nil, nil
	}

	switch filter.Type() {
	case domainusage.FilterTypeGroup:
		parts := []exp.Expression{}
		for _, rule := range filter.Rules() {
			part, err := filterPredicateExpression(rule)
			if err != nil {
				return nil, err
			}
			if part == nil {
				continue
			}
			parts = append(parts, part)
		}
		if len(parts) == 0 {
			return nil, nil
		}
		if filter.GroupOp() == domainusage.FilterGroupOr {
			return goqu.Or(parts...), nil
		}
		return goqu.And(parts...), nil
	case domainusage.FilterTypeCondition:
		return filterConditionPredicateExpression(filter)
	default:
		return nil, nil
	}
}

type sqlFilterField struct {
	expression       sqlOperand
	existsExpression exp.Expression
	valueKind        string
	metadataKey      string
}

func filterConditionPredicateExpression(filter domainusage.Filter) (exp.Expression, error) {
	field, err := filterFieldExpression(filter.Field())
	if err != nil {
		return nil, err
	}

	op := filter.ConditionOp()
	if op == domainusage.FilterOpExists {
		if field.existsExpression != nil {
			return field.existsExpression, nil
		}
		return field.expression.IsNotNull(), nil
	}

	switch op {
	case domainusage.FilterOpEqual:
		if field.metadataKey != "" {
			return metadataContainsPredicateExpression(field.metadataKey, filter.Value())
		}
		value, err := sqlFilterValue(filter.Value(), field.valueKind)
		if err != nil {
			return nil, err
		}
		return field.expression.Eq(value), nil
	case domainusage.FilterOpNotEqual:
		value, err := sqlFilterValue(filter.Value(), field.valueKind)
		if err != nil {
			return nil, err
		}
		return field.expression.Neq(value), nil
	case domainusage.FilterOpGreaterThan:
		value, err := sqlFilterValue(filter.Value(), field.valueKind)
		if err != nil {
			return nil, err
		}
		return field.expression.Gt(value), nil
	case domainusage.FilterOpGreaterThanOrEqual:
		value, err := sqlFilterValue(filter.Value(), field.valueKind)
		if err != nil {
			return nil, err
		}
		return field.expression.Gte(value), nil
	case domainusage.FilterOpLessThan:
		value, err := sqlFilterValue(filter.Value(), field.valueKind)
		if err != nil {
			return nil, err
		}
		return field.expression.Lt(value), nil
	case domainusage.FilterOpLessThanOrEqual:
		value, err := sqlFilterValue(filter.Value(), field.valueKind)
		if err != nil {
			return nil, err
		}
		return field.expression.Lte(value), nil
	case domainusage.FilterOpIn:
		values, ok := filter.Value().([]any)
		if !ok || len(values) == 0 {
			return nil, fmt.Errorf("%w: invalid in filter value", domain.ErrInvalidInput)
		}
		if field.metadataKey != "" {
			parts := make([]exp.Expression, 0, len(values))
			for _, raw := range values {
				part, err := metadataContainsPredicateExpression(field.metadataKey, raw)
				if err != nil {
					return nil, err
				}
				parts = append(parts, part)
			}
			return goqu.Or(parts...), nil
		}
		sqlValues := make([]interface{}, 0, len(values))
		for _, raw := range values {
			value, err := sqlFilterValue(raw, field.valueKind)
			if err != nil {
				return nil, err
			}
			sqlValues = append(sqlValues, value)
		}
		return field.expression.In(sqlValues...), nil
	case domainusage.FilterOpContains:
		value, err := sqlFilterValue(filter.Value(), "text")
		if err != nil {
			return nil, err
		}
		return goqu.L("CAST(? AS TEXT)", field.expression).Like("%" + fmt.Sprint(value) + "%"), nil
	default:
		return nil, fmt.Errorf("%w: unsupported filter operator %q", domain.ErrInvalidInput, op)
	}
}

func metadataContainsPredicateExpression(key string, value any) (exp.Expression, error) {
	payload, err := metadataContainsPayload(key, value)
	if err != nil {
		return nil, err
	}
	return goqu.L("metadata @> ?::jsonb", payload), nil
}

func filterFieldExpression(field string) (sqlFilterField, error) {
	switch field {
	case "subject":
		return sqlFilterField{expression: goqu.C("subject"), valueKind: "text"}, nil
	case "meter":
		return sqlFilterField{expression: goqu.C("meter_name"), valueKind: "text"}, nil
	case "quantity":
		return sqlFilterField{expression: goqu.C("quantity"), valueKind: "number"}, nil
	case "timestamp", "event_time":
		return sqlFilterField{expression: goqu.C("event_time"), valueKind: "time"}, nil
	case "received_at":
		return sqlFilterField{expression: goqu.C("received_at"), valueKind: "time"}, nil
	case "idempotency_key":
		return sqlFilterField{expression: goqu.C("idempotency_key"), valueKind: "text"}, nil
	default:
		key := strings.TrimPrefix(field, "metadata.")
		if key == field {
			return sqlFilterField{}, fmt.Errorf("%w: unsupported filter field %q", domain.ErrInvalidInput, field)
		}
		expression, err := metadataTextExpression(key)
		if err != nil {
			return sqlFilterField{}, fmt.Errorf("%w: unsupported filter field %q", domain.ErrInvalidInput, field)
		}
		return sqlFilterField{
			expression:       expression,
			existsExpression: goqu.L("metadata #> string_to_array(?, '.') IS NOT NULL", key),
			valueKind:        "text",
			metadataKey:      key,
		}, nil
	}
}

func metadataTextExpression(key string) (exp.LiteralExpression, error) {
	if !metadataKeyPattern.MatchString(key) {
		return nil, fmt.Errorf("%w: unsupported metadata key %q", domain.ErrInvalidInput, key)
	}
	return goqu.L("metadata #>> string_to_array(?, '.')", key), nil
}

func metadataContainsPayload(key string, value any) (string, error) {
	if !metadataKeyPattern.MatchString(key) {
		return "", fmt.Errorf("%w: unsupported metadata filter key %q", domain.ErrInvalidInput, key)
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
