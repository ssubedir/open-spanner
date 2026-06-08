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

	MaxHourRange  = 31 * 24 * time.Hour
	MaxDayRange   = 366 * 24 * time.Hour
	MaxMonthYears = 5
)

type Query struct {
	subject     string
	meterName   string
	from        time.Time
	to          time.Time
	bucketSize  BucketSize
	aggregation domainmeter.Aggregation
	metadata    map[string]string
	groupBy     string
	limit       int
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
	subject = strings.TrimSpace(subject)
	meterName = strings.TrimSpace(meterName)
	groupBy = strings.TrimSpace(groupBy)

	if subject == "" {
		return Query{}, fmt.Errorf("%w: subject is required", domain.ErrInvalidInput)
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
		groupBy:     groupBy,
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
	return q.groupBy
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
