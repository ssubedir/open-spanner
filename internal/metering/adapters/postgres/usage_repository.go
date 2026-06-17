package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/postgres/postgresdb"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

var metadataKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)

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

		err = queriesFor(txCtx, r.queries).SaveBulkUsageIngestion(txCtx, postgresdb.SaveBulkUsageIngestionParams{
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

	err = queriesFor(ctx, r.queries).SaveUsageEvent(ctx, postgresdb.SaveUsageEventParams{
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
	if !bucketQueryNeedsDynamicSQL(query) {
		return r.queryBucketsWithGeneratedSQL(ctx, query)
	}

	return r.queryBucketsWithDynamicSQL(ctx, query)
}

func bucketQueryNeedsDynamicSQL(query domainusage.Query) bool {
	return !query.Filter().IsZero() || len(query.Metadata()) > 0 || len(query.GroupByFields()) > 0
}

func (r *UsageRepository) queryBucketsWithGeneratedSQL(ctx context.Context, query domainusage.Query) ([]domainusage.Bucket, error) {
	rows, err := queriesFor(ctx, r.queries).ListUsageBuckets(ctx, postgresdb.ListUsageBucketsParams{
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
	if !metadataKeyPattern.MatchString(query.Field()) {
		return nil, fmt.Errorf("%w: unsupported metadata field %q", domain.ErrInvalidInput, query.Field())
	}

	rows, err := queriesFor(ctx, r.queries).ListUsageDimensionValues(ctx, postgresdb.ListUsageDimensionValuesParams{
		Field:     query.Field(),
		MeterName: query.MeterName(),
		Subject:   eventStringValue(query.Subject()),
		FromTime:  eventTimeValue(query.From()),
		ToTime:    eventTimeValue(query.To()),
		Limit:     int32(query.Limit()),
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
		return r.findBreakdownWithGeneratedSQL(ctx, query)
	}

	return r.findBreakdownWithDynamicSQL(ctx, query)
}

func (r *UsageRepository) findBreakdownWithGeneratedSQL(ctx context.Context, query domainusage.BreakdownQuery) ([]domainusage.BreakdownItem, error) {
	if _, err := breakdownFieldExpression(query.Field()); err != nil {
		return nil, err
	}

	rows, err := queriesFor(ctx, r.queries).ListUsageBreakdown(ctx, postgresdb.ListUsageBreakdownParams{
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
	cursorEventTime, cursorID := eventCursorValues(query.Cursor())
	rows, err := queriesFor(ctx, r.queries).ListUsageEvents(ctx, postgresdb.ListUsageEventsParams{
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
	rows, err := queriesFor(ctx, r.queries).ListUsageSubjectStats(ctx, postgresdb.ListUsageSubjectStatsParams{
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
	total := 0
	for {
		deleted, err := queriesFor(ctx, r.queries).PruneUsageEventsBatch(ctx, postgresdb.PruneUsageEventsBatchParams{
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
	count, err := queriesFor(ctx, r.queries).CountPrunableUsageEvents(ctx, postgresdb.CountPrunableUsageEventsParams{
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

	err = queriesFor(ctx, r.queries).SaveUsagePruneRun(ctx, postgresdb.SaveUsagePruneRunParams{
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
	rows, err := queriesFor(ctx, r.queries).ListUsagePruneRuns(ctx, postgresdb.ListUsagePruneRunsParams{
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
	count, err := queriesFor(ctx, r.queries).CountUsagePruneRuns(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *UsageRepository) SaveIngestionRun(ctx context.Context, run domainusage.IngestionRun) (domainusage.IngestionRun, error) {
	err := queriesFor(ctx, r.queries).SaveUsageIngestionRun(ctx, postgresdb.SaveUsageIngestionRunParams{
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
	rows, err := queriesFor(ctx, r.queries).ListUsageIngestionRuns(ctx, postgresdb.ListUsageIngestionRunsParams{
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
		ID:          job.ID(),
		Kind:        string(job.Kind()),
		Status:      string(job.Status()),
		Format:      string(job.Format()),
		QueryJson:   job.QueryJSON(),
		Error:       job.ErrorMessage(),
		CreatedAt:   formatTime(job.CreatedAt()),
		UpdatedAt:   formatTime(job.UpdatedAt()),
		CompletedAt: exportJobTimeValue(job.CompletedAt()),
	})
	if err != nil {
		return domainusage.ExportJob{}, err
	}

	return job, nil
}

func (r *UsageRepository) FindExportJob(ctx context.Context, id string) (domainusage.ExportJob, error) {
	row, err := queriesFor(ctx, r.queries).FindUsageExportJob(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainusage.ExportJob{}, domain.ErrNotFound
		}
		return domainusage.ExportJob{}, err
	}

	return exportJobFromFields(row.ID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, row.CreatedAt, row.UpdatedAt, row.CompletedAt)
}

func (r *UsageRepository) FindExportJobs(ctx context.Context, query domainusage.RunQuery) ([]domainusage.ExportJob, error) {
	cursorCreatedAt, cursorID := runCursorValues(query)
	rows, err := queriesFor(ctx, r.queries).ListUsageExportJobs(ctx, postgresdb.ListUsageExportJobsParams{
		CursorCreatedAt: cursorCreatedAt,
		CursorID:        cursorID,
		Limit:           int32(query.Limit()),
	})
	if err != nil {
		return nil, err
	}

	jobs := make([]domainusage.ExportJob, 0, len(rows))
	for _, row := range rows {
		job, err := exportJobFromFields(row.ID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, row.CreatedAt, row.UpdatedAt, row.CompletedAt)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
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

func exportJobFromFields(id string, kind string, status string, format string, queryJSON string, errorMessage string, createdAtText string, updatedAtText string, completedAtText sql.NullString) (domainusage.ExportJob, error) {
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

	return domainusage.NewExportJob(
		id,
		domainusage.ExportJobKind(kind),
		domainusage.ExportJobStatus(status),
		domainusage.ExportJobFormat(format),
		queryJSON,
		errorMessage,
		createdAt,
		updatedAt,
		completedAt,
	)
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
