package usage

import (
	"fmt"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
)

type BucketSize string

const (
	BucketHour  BucketSize = "hour"
	BucketDay   BucketSize = "day"
	BucketMonth BucketSize = "month"

	DefaultLimit = 100
	MaxLimit     = 1000
	MaxGroupBy   = 5

	MaxHourRange  = 31 * 24 * time.Hour
	MaxDayRange   = 366 * 24 * time.Hour
	MaxMonthYears = 5

	GroupBySubject = "subject"
)

type Query struct {
	subject     string
	meterName   string
	from        time.Time
	to          time.Time
	bucketSize  BucketSize
	aggregation domainmeter.Aggregation
	metadata    map[string]string
	groupBy     []string
	limit       int
	filter      Filter
}

type AggregateQuery struct {
	subject     string
	meterName   string
	from        time.Time
	to          time.Time
	aggregation domainmeter.Aggregation
	metadata    map[string]string
	filter      Filter
}

type EventQuery struct {
	subject   string
	meterName string
	from      time.Time
	to        time.Time
	limit     int
	cursor    EventCursor
	filter    Filter
}

type DimensionValueQuery struct {
	meterName string
	field     string
	subject   string
	from      time.Time
	to        time.Time
	limit     int
}

type BreakdownQuery struct {
	meterName   string
	field       string
	subject     string
	from        time.Time
	to          time.Time
	aggregation domainmeter.Aggregation
	limit       int
	filter      Filter
}

type EventCursor struct {
	eventTime time.Time
	id        string
}

type EventPage struct {
	events     []Event
	nextCursor EventCursor
}

type PruneQuery struct {
	meterName string
	before    time.Time
}

type SubjectStatsQuery struct {
	limit       int
	lastEventAt time.Time
	subject     string
}

type RunQuery struct {
	limit     int
	createdAt time.Time
	id        string
}

type Bucket struct {
	subject     string
	meterName   string
	bucketSize  BucketSize
	bucketStart time.Time
	quantity    float64
	group       map[string]string
}

type DimensionValue struct {
	field       string
	value       string
	usageEvents int
}

type BreakdownItem struct {
	field       string
	value       string
	quantity    float64
	usageEvents int
}

type Aggregate struct {
	quantity    float64
	usageEvents int
}

func NewPruneQuery(meterName string, before time.Time) (PruneQuery, error) {
	meterName = strings.TrimSpace(meterName)
	if meterName == "" {
		return PruneQuery{}, fmt.Errorf("%w: meter is required", domain.ErrInvalidInput)
	}
	if before.IsZero() {
		return PruneQuery{}, fmt.Errorf("%w: prune cutoff is required", domain.ErrInvalidInput)
	}

	return PruneQuery{meterName: meterName, before: before.UTC()}, nil
}

func NewEventQuery(subject, meterName string, from, to time.Time, limit int, cursor EventCursor) (EventQuery, error) {
	return NewFilteredEventQuery(subject, meterName, from, to, limit, cursor, EmptyFilter())
}

func NewFilteredEventQuery(subject, meterName string, from, to time.Time, limit int, cursor EventCursor, filter Filter) (EventQuery, error) {
	subject = strings.TrimSpace(subject)
	meterName = strings.TrimSpace(meterName)

	if !from.IsZero() && !to.IsZero() && !from.Before(to) {
		return EventQuery{}, fmt.Errorf("%w: valid from and to range is required", domain.ErrInvalidInput)
	}

	return EventQuery{
		subject:   subject,
		meterName: meterName,
		from:      from.UTC(),
		to:        to.UTC(),
		limit:     NormalizeLimit(limit),
		cursor:    cursor,
		filter:    filter,
	}, nil
}

func NewEventCursor(eventTime time.Time, id string) (EventCursor, error) {
	id = strings.TrimSpace(id)
	if eventTime.IsZero() {
		return EventCursor{}, fmt.Errorf("%w: cursor event time is required", domain.ErrInvalidInput)
	}
	if id == "" {
		return EventCursor{}, fmt.Errorf("%w: cursor id is required", domain.ErrInvalidInput)
	}

	return EventCursor{eventTime: eventTime.UTC(), id: id}, nil
}

func NewEventPage(events []Event, limit int) EventPage {
	limit = NormalizeLimit(limit)
	hasNext := len(events) > limit
	if hasNext {
		events = events[:limit]
	}

	page := EventPage{events: events}
	if hasNext && len(events) > 0 {
		last := events[len(events)-1]
		page.nextCursor = EventCursor{eventTime: last.EventTime(), id: last.ID()}
	}

	return page
}

func NewDimensionValueQuery(meterName string, field string, subject string, from time.Time, to time.Time, limit int) (DimensionValueQuery, error) {
	meterName = strings.TrimSpace(meterName)
	field = strings.TrimPrefix(strings.TrimSpace(field), "metadata.")
	subject = strings.TrimSpace(subject)

	if meterName == "" {
		return DimensionValueQuery{}, fmt.Errorf("%w: meter is required", domain.ErrInvalidInput)
	}
	if field == "" {
		return DimensionValueQuery{}, fmt.Errorf("%w: dimension field is required", domain.ErrInvalidInput)
	}
	if !from.IsZero() && !to.IsZero() && !from.Before(to) {
		return DimensionValueQuery{}, fmt.Errorf("%w: valid from and to range is required", domain.ErrInvalidInput)
	}

	return DimensionValueQuery{
		meterName: meterName,
		field:     field,
		subject:   subject,
		from:      from.UTC(),
		to:        to.UTC(),
		limit:     NormalizeLimit(limit),
	}, nil
}

func NewDimensionValue(field string, value string, usageEvents int) DimensionValue {
	return DimensionValue{
		field:       field,
		value:       value,
		usageEvents: usageEvents,
	}
}

func NewBreakdownQuery(meterName string, field string, subject string, from time.Time, to time.Time, aggregation domainmeter.Aggregation, limit int, filter Filter) (BreakdownQuery, error) {
	meterName = strings.TrimSpace(meterName)
	field = strings.TrimPrefix(strings.TrimSpace(field), "metadata.")
	subject = strings.TrimSpace(subject)

	if meterName == "" {
		return BreakdownQuery{}, fmt.Errorf("%w: meter is required", domain.ErrInvalidInput)
	}
	if field == "" {
		return BreakdownQuery{}, fmt.Errorf("%w: breakdown field is required", domain.ErrInvalidInput)
	}
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return BreakdownQuery{}, fmt.Errorf("%w: valid from and to range is required", domain.ErrInvalidInput)
	}
	if aggregation == "" {
		aggregation = domainmeter.AggregationSum
	}
	if !domainmeter.IsSupportedAggregation(aggregation) {
		return BreakdownQuery{}, fmt.Errorf("%w: unsupported aggregation %q", domain.ErrInvalidInput, aggregation)
	}

	return BreakdownQuery{
		meterName:   meterName,
		field:       field,
		subject:     subject,
		from:        from.UTC(),
		to:          to.UTC(),
		aggregation: aggregation,
		limit:       NormalizeLimit(limit),
		filter:      filter,
	}, nil
}

func NewBreakdownItem(field string, value string, quantity float64, usageEvents int) BreakdownItem {
	return BreakdownItem{
		field:       field,
		value:       value,
		quantity:    quantity,
		usageEvents: usageEvents,
	}
}

func NewAggregateQuery(subject, meterName string, from, to time.Time, aggregation domainmeter.Aggregation, metadata map[string]string, filter Filter) (AggregateQuery, error) {
	subject = strings.TrimSpace(subject)
	meterName = strings.TrimSpace(meterName)

	if meterName == "" {
		return AggregateQuery{}, fmt.Errorf("%w: meter is required", domain.ErrInvalidInput)
	}
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return AggregateQuery{}, fmt.Errorf("%w: valid from and to range is required", domain.ErrInvalidInput)
	}
	if aggregation == "" {
		aggregation = domainmeter.AggregationSum
	}
	if !domainmeter.IsSupportedAggregation(aggregation) {
		return AggregateQuery{}, fmt.Errorf("%w: unsupported aggregation %q", domain.ErrInvalidInput, aggregation)
	}

	metadataFilters := map[string]string{}
	for key, value := range metadata {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return AggregateQuery{}, fmt.Errorf("%w: metadata filter key is required", domain.ErrInvalidInput)
		}
		if value == "" {
			return AggregateQuery{}, fmt.Errorf("%w: metadata filter value is required", domain.ErrInvalidInput)
		}
		metadataFilters[key] = value
	}

	return AggregateQuery{
		subject:     subject,
		meterName:   meterName,
		from:        from.UTC(),
		to:          to.UTC(),
		aggregation: aggregation,
		metadata:    metadataFilters,
		filter:      filter,
	}, nil
}

func NewAggregate(quantity float64, usageEvents int) Aggregate {
	return Aggregate{quantity: quantity, usageEvents: usageEvents}
}

func NewSubjectStatsQuery(limit int, lastEventAt time.Time, subject string) SubjectStatsQuery {
	return SubjectStatsQuery{
		limit:       NormalizeLimit(limit),
		lastEventAt: lastEventAt.UTC(),
		subject:     strings.TrimSpace(subject),
	}
}

func NewRunQuery(limit int, createdAt time.Time, id string) RunQuery {
	return RunQuery{
		limit:     NormalizeLimit(limit),
		createdAt: createdAt.UTC(),
		id:        strings.TrimSpace(id),
	}
}

func NewQuery(subject, meterName string, from, to time.Time, bucketSize BucketSize, aggregation domainmeter.Aggregation, metadata map[string]string, groupBy string, limit int) (Query, error) {
	return NewFilteredQuery(subject, meterName, from, to, bucketSize, aggregation, metadata, groupBy, limit, EmptyFilter())
}

func NewFilteredQuery(subject, meterName string, from, to time.Time, bucketSize BucketSize, aggregation domainmeter.Aggregation, metadata map[string]string, groupBy string, limit int, filter Filter) (Query, error) {
	return NewGroupedFilteredQuery(subject, meterName, from, to, bucketSize, aggregation, metadata, SplitGroupBy(groupBy), limit, filter)
}

func NewGroupedQuery(subject, meterName string, from, to time.Time, bucketSize BucketSize, aggregation domainmeter.Aggregation, metadata map[string]string, groupBy []string, limit int) (Query, error) {
	return NewGroupedFilteredQuery(subject, meterName, from, to, bucketSize, aggregation, metadata, groupBy, limit, EmptyFilter())
}

func NewGroupedFilteredQuery(subject, meterName string, from, to time.Time, bucketSize BucketSize, aggregation domainmeter.Aggregation, metadata map[string]string, groupBy []string, limit int, filter Filter) (Query, error) {
	subject = strings.TrimSpace(subject)
	meterName = strings.TrimSpace(meterName)
	groupByFields, err := NormalizeGroupBy(groupBy)
	if err != nil {
		return Query{}, err
	}

	if meterName == "" {
		return Query{}, fmt.Errorf("%w: meter is required", domain.ErrInvalidInput)
	}
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return Query{}, fmt.Errorf("%w: valid from and to range is required", domain.ErrInvalidInput)
	}
	if bucketSize == "" {
		bucketSize = BucketDay
	}
	switch bucketSize {
	case BucketHour, BucketDay, BucketMonth:
	default:
		return Query{}, fmt.Errorf("%w: unsupported bucket size %q", domain.ErrInvalidInput, bucketSize)
	}
	if err := validateRange(from.UTC(), to.UTC(), bucketSize); err != nil {
		return Query{}, err
	}
	if aggregation == "" {
		aggregation = domainmeter.AggregationSum
	}
	if !domainmeter.IsSupportedAggregation(aggregation) {
		return Query{}, fmt.Errorf("%w: unsupported aggregation %q", domain.ErrInvalidInput, aggregation)
	}
	metadataFilters := map[string]string{}
	for key, value := range metadata {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return Query{}, fmt.Errorf("%w: metadata filter key is required", domain.ErrInvalidInput)
		}
		if value == "" {
			return Query{}, fmt.Errorf("%w: metadata filter value is required", domain.ErrInvalidInput)
		}
		metadataFilters[key] = value
	}
	limit = NormalizeLimit(limit)

	return Query{
		subject:     subject,
		meterName:   meterName,
		from:        from.UTC(),
		to:          to.UTC(),
		bucketSize:  bucketSize,
		aggregation: aggregation,
		metadata:    metadataFilters,
		groupBy:     groupByFields,
		limit:       limit,
		filter:      filter,
	}, nil
}

func validateRange(from, to time.Time, bucketSize BucketSize) error {
	switch bucketSize {
	case BucketHour:
		if to.Sub(from) > MaxHourRange {
			return fmt.Errorf("%w: hour bucket range cannot exceed 31 days", domain.ErrInvalidInput)
		}
	case BucketDay:
		if to.Sub(from) > MaxDayRange {
			return fmt.Errorf("%w: day bucket range cannot exceed 366 days", domain.ErrInvalidInput)
		}
	case BucketMonth:
		if to.After(from.AddDate(MaxMonthYears, 0, 0)) {
			return fmt.Errorf("%w: month bucket range cannot exceed 5 years", domain.ErrInvalidInput)
		}
	}
	return nil
}

func NormalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}

func SplitGroupBy(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	fields := make([]string, 0, len(parts))
	for _, part := range parts {
		fields = append(fields, part)
	}
	return fields
}

func SplitGroupByValues(values []string) []string {
	fields := []string{}
	for _, value := range values {
		fields = append(fields, SplitGroupBy(value)...)
	}
	return fields
}

func NormalizeGroupBy(fields []string) ([]string, error) {
	normalized := []string{}
	seen := map[string]struct{}{}
	for _, field := range fields {
		for _, part := range SplitGroupBy(field) {
			key := strings.TrimSpace(part)
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			normalized = append(normalized, key)
		}
	}
	if len(normalized) > MaxGroupBy {
		return nil, fmt.Errorf("%w: group_by supports up to %d fields", domain.ErrInvalidInput, MaxGroupBy)
	}
	return normalized, nil
}

func IsSubjectGroupBy(field string) bool {
	return strings.TrimSpace(field) == GroupBySubject
}

func NewBucket(subject, meterName string, bucketSize BucketSize, bucketStart time.Time, quantity float64) Bucket {
	return NewBucketWithGroup(subject, meterName, bucketSize, bucketStart, quantity, nil)
}

func NewBucketWithGroup(subject, meterName string, bucketSize BucketSize, bucketStart time.Time, quantity float64, group map[string]string) Bucket {
	groupCopy := map[string]string{}
	for key, value := range group {
		groupCopy[key] = value
	}

	return Bucket{
		subject:     subject,
		meterName:   meterName,
		bucketSize:  bucketSize,
		bucketStart: bucketStart.UTC(),
		quantity:    quantity,
		group:       groupCopy,
	}
}

func (q EventQuery) Subject() string {
	return q.subject
}

func (q EventQuery) MeterName() string {
	return q.meterName
}

func (q EventQuery) From() time.Time {
	return q.from
}

func (q EventQuery) To() time.Time {
	return q.to
}

func (q EventQuery) Limit() int {
	return q.limit
}

func (q EventQuery) Cursor() EventCursor {
	return q.cursor
}

func (q EventQuery) Filter() Filter {
	return q.filter
}

func (q PruneQuery) MeterName() string {
	return q.meterName
}

func (q PruneQuery) Before() time.Time {
	return q.before
}

func (q SubjectStatsQuery) Limit() int {
	return q.limit
}

func (q SubjectStatsQuery) LastEventAt() time.Time {
	return q.lastEventAt
}

func (q SubjectStatsQuery) Subject() string {
	return q.subject
}

func (q SubjectStatsQuery) HasCursor() bool {
	return !q.lastEventAt.IsZero() && q.subject != ""
}

func (q RunQuery) Limit() int {
	return q.limit
}

func (q RunQuery) CreatedAt() time.Time {
	return q.createdAt
}

func (q RunQuery) ID() string {
	return q.id
}

func (q RunQuery) HasCursor() bool {
	return !q.createdAt.IsZero() && q.id != ""
}

func (q DimensionValueQuery) MeterName() string {
	return q.meterName
}

func (q DimensionValueQuery) Field() string {
	return q.field
}

func (q DimensionValueQuery) Subject() string {
	return q.subject
}

func (q DimensionValueQuery) From() time.Time {
	return q.from
}

func (q DimensionValueQuery) To() time.Time {
	return q.to
}

func (q DimensionValueQuery) Limit() int {
	return q.limit
}

func (q BreakdownQuery) MeterName() string {
	return q.meterName
}

func (q BreakdownQuery) Field() string {
	return q.field
}

func (q BreakdownQuery) Subject() string {
	return q.subject
}

func (q BreakdownQuery) From() time.Time {
	return q.from
}

func (q BreakdownQuery) To() time.Time {
	return q.to
}

func (q BreakdownQuery) Aggregation() domainmeter.Aggregation {
	return q.aggregation
}

func (q BreakdownQuery) Limit() int {
	return q.limit
}

func (q BreakdownQuery) Filter() Filter {
	return q.filter
}

func (q AggregateQuery) Subject() string {
	return q.subject
}

func (q AggregateQuery) MeterName() string {
	return q.meterName
}

func (q AggregateQuery) From() time.Time {
	return q.from
}

func (q AggregateQuery) To() time.Time {
	return q.to
}

func (q AggregateQuery) Aggregation() domainmeter.Aggregation {
	return q.aggregation
}

func (q AggregateQuery) Metadata() map[string]string {
	metadata := make(map[string]string, len(q.metadata))
	for key, value := range q.metadata {
		metadata[key] = value
	}
	return metadata
}

func (q AggregateQuery) Filter() Filter {
	return q.filter
}

func (c EventCursor) EventTime() time.Time {
	return c.eventTime
}

func (c EventCursor) ID() string {
	return c.id
}

func (c EventCursor) IsZero() bool {
	return c.eventTime.IsZero() && c.id == ""
}

func (p EventPage) Events() []Event {
	events := make([]Event, len(p.events))
	copy(events, p.events)
	return events
}

func (p EventPage) NextCursor() EventCursor {
	return p.nextCursor
}

func (q Query) Subject() string {
	return q.subject
}

func (q Query) MeterName() string {
	return q.meterName
}

func (q Query) From() time.Time {
	return q.from
}

func (q Query) To() time.Time {
	return q.to
}

func (q Query) BucketSize() BucketSize {
	return q.bucketSize
}

func (q Query) Aggregation() domainmeter.Aggregation {
	return q.aggregation
}

func (q Query) Metadata() map[string]string {
	metadata := make(map[string]string, len(q.metadata))
	for key, value := range q.metadata {
		metadata[key] = value
	}
	return metadata
}

func (q Query) GroupBy() string {
	if len(q.groupBy) == 0 {
		return ""
	}
	return q.groupBy[0]
}

func (q Query) GroupByFields() []string {
	fields := make([]string, len(q.groupBy))
	copy(fields, q.groupBy)
	return fields
}

func (q Query) Limit() int {
	return q.limit
}

func (q Query) Filter() Filter {
	return q.filter
}

func (b Bucket) Subject() string {
	return b.subject
}

func (b Bucket) MeterName() string {
	return b.meterName
}

func (b Bucket) BucketSize() BucketSize {
	return b.bucketSize
}

func (b Bucket) BucketStart() time.Time {
	return b.bucketStart
}

func (b Bucket) Quantity() float64 {
	return b.quantity
}

func (b Bucket) Group() map[string]string {
	group := make(map[string]string, len(b.group))
	for key, value := range b.group {
		group[key] = value
	}
	return group
}

func (v DimensionValue) Field() string {
	return v.field
}

func (v DimensionValue) Value() string {
	return v.value
}

func (v DimensionValue) UsageEvents() int {
	return v.usageEvents
}

func (b BreakdownItem) Field() string {
	return b.field
}

func (b BreakdownItem) Value() string {
	return b.value
}

func (b BreakdownItem) Quantity() float64 {
	return b.quantity
}

func (b BreakdownItem) UsageEvents() int {
	return b.usageEvents
}

func (a Aggregate) Quantity() float64 {
	return a.quantity
}

func (a Aggregate) UsageEvents() int {
	return a.usageEvents
}
