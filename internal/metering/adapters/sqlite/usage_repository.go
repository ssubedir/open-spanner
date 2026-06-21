package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite/sqlitedb"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

var metadataKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)

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
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.BulkSaveResult{}, err
	}
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
	err = r.store.WithinTransaction(ctx, func(txCtx context.Context) error {
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
			WorkspaceID:    workspaceID,
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
	var saved domainusage.Event
	err := r.store.WithinTransaction(ctx, func(txCtx context.Context) error {
		var err error
		saved, _, err = r.saveWithDuplicate(txCtx, event)
		return err
	})
	return saved, err
}

func (r *UsageRepository) saveWithDuplicate(ctx context.Context, event domainusage.Event) (domainusage.Event, bool, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.Event{}, false, err
	}
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
		WorkspaceID:    workspaceID,
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

	if err := queriesFor(ctx, r.queries).IncrementWorkspaceUsageEvents(ctx, sqlitedb.IncrementWorkspaceUsageEventsParams{
		WorkspaceID: workspaceID,
		Delta:       1,
		UpdatedAt:   formatTime(time.Now().UTC()),
	}); err != nil {
		return domainusage.Event{}, false, err
	}
	if err := r.incrementEntitlementUsageCounters(ctx, workspaceID, event); err != nil {
		return domainusage.Event{}, false, err
	}

	return event, false, nil
}

func (r *UsageRepository) incrementEntitlementUsageCounters(ctx context.Context, workspaceID string, event domainusage.Event) error {
	updatedAt := formatTime(time.Now().UTC())
	for _, window := range entitlementCounterWindows(event.EventTime()) {
		if err := queriesFor(ctx, r.queries).IncrementEntitlementUsageCounter(ctx, sqlitedb.IncrementEntitlementUsageCounterParams{
			WorkspaceID: workspaceID,
			Subject:     event.Subject(),
			MeterName:   event.MeterName(),
			Period:      window.period,
			PeriodStart: formatTime(window.from),
			PeriodEnd:   formatTime(window.to),
			Quantity:    event.Quantity(),
			EventTime:   formatTime(event.EventTime()),
			UpdatedAt:   updatedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

type entitlementCounterWindow struct {
	period string
	from   time.Time
	to     time.Time
}

func entitlementCounterWindows(at time.Time) []entitlementCounterWindow {
	at = at.UTC()
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	weekday := int(day.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	week := day.AddDate(0, 0, -(weekday - 1))
	month := time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)
	year := time.Date(at.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)

	return []entitlementCounterWindow{
		{period: "day", from: day, to: day.AddDate(0, 0, 1)},
		{period: "week", from: week, to: week.AddDate(0, 0, 7)},
		{period: "month", from: month, to: month.AddDate(0, 1, 0)},
		{period: "year", from: year, to: year.AddDate(1, 0, 0)},
	}
}

func (r *UsageRepository) Query(ctx context.Context, query domainusage.Query) ([]domainusage.Bucket, error) {
	if !bucketQueryNeedsDynamicSQL(query) {
		return r.queryBucketsWithGeneratedSQL(ctx, query)
	}

	return r.queryBucketsWithDynamicSQL(ctx, query)
}

func (r *UsageRepository) Aggregate(ctx context.Context, query domainusage.AggregateQuery) (domainusage.Aggregate, error) {
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
	rows, err := queriesFor(ctx, r.queries).ListUsageBuckets(ctx, sqlitedb.ListUsageBucketsParams{
		WorkspaceID: workspaceID,
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

func (r *UsageRepository) FindDimensionValues(ctx context.Context, query domainusage.DimensionValueQuery) ([]domainusage.DimensionValue, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	path, err := sqliteJSONPath(query.Field())
	if err != nil {
		return nil, err
	}

	rows, err := queriesFor(ctx, r.queries).ListUsageDimensionValues(ctx, sqlitedb.ListUsageDimensionValuesParams{
		WorkspaceID: workspaceID,
		Path:        path,
		MeterName:   query.MeterName(),
		Subject:     eventStringValue(query.Subject()),
		FromTime:    eventTimeValue(query.From()),
		ToTime:      eventTimeValue(query.To()),
		Limit:       int64(query.Limit()),
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
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	path, err := sqliteJSONPath(query.Field())
	if err != nil {
		return nil, err
	}

	rows, err := queriesFor(ctx, r.queries).ListUsageBreakdown(ctx, sqlitedb.ListUsageBreakdownParams{
		WorkspaceID:     workspaceID,
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
	rows, err := queriesFor(ctx, r.queries).ListUsageEvents(ctx, sqlitedb.ListUsageEventsParams{
		WorkspaceID:     workspaceID,
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
	rows, err := queriesFor(ctx, r.queries).ListUsageSubjectStats(ctx, sqlitedb.ListUsageSubjectStatsParams{
		WorkspaceID:       workspaceID,
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
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return 0, err
	}
	deleted, err := queriesFor(ctx, r.queries).PruneUsageEvents(ctx, sqlitedb.PruneUsageEventsParams{
		WorkspaceID: workspaceID,
		MeterName:   query.MeterName(),
		EventTime:   formatTime(query.Before()),
	})
	if err != nil {
		return 0, err
	}
	if deleted > 0 {
		if err := queriesFor(ctx, r.queries).IncrementWorkspaceUsageEvents(ctx, sqlitedb.IncrementWorkspaceUsageEventsParams{
			WorkspaceID: workspaceID,
			Delta:       -deleted,
			UpdatedAt:   formatTime(time.Now().UTC()),
		}); err != nil {
			return 0, err
		}
	}
	return int(deleted), nil
}

func (r *UsageRepository) CountPrunableEvents(ctx context.Context, query domainusage.PruneQuery) (int, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return 0, err
	}
	count, err := queriesFor(ctx, r.queries).CountPrunableUsageEvents(ctx, sqlitedb.CountPrunableUsageEventsParams{
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

	err = queriesFor(ctx, r.queries).SaveUsagePruneRun(ctx, sqlitedb.SaveUsagePruneRunParams{
		ID:          run.ID(),
		WorkspaceID: workspaceID,
		DryRun:      int64(boolInt(run.DryRun())),
		Deleted:     int64(run.Deleted()),
		Meters:      meters,
		CreatedAt:   formatTime(run.CreatedAt()),
	})
	if err != nil {
		return domainusage.PruneRun{}, err
	}

	if err := queriesFor(ctx, r.queries).IncrementWorkspacePruneRuns(ctx, sqlitedb.IncrementWorkspacePruneRunsParams{
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
	rows, err := queriesFor(ctx, r.queries).ListUsagePruneRuns(ctx, sqlitedb.ListUsagePruneRunsParams{
		WorkspaceID:     workspaceID,
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
	err = queriesFor(ctx, r.queries).SaveUsageIngestionRun(ctx, sqlitedb.SaveUsageIngestionRunParams{
		ID:          run.ID(),
		WorkspaceID: workspaceID,
		Kind:        string(run.Kind()),
		Accepted:    int64(run.Accepted()),
		Duplicates:  int64(run.Duplicates()),
		Failed:      int64(run.Failed()),
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
	rows, err := queriesFor(ctx, r.queries).ListUsageIngestionRuns(ctx, sqlitedb.ListUsageIngestionRunsParams{
		WorkspaceID:     workspaceID,
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

func (r *UsageRepository) SaveExportJob(ctx context.Context, job domainusage.ExportJob) (domainusage.ExportJob, error) {
	err := queriesFor(ctx, r.queries).SaveUsageExportJob(ctx, sqlitedb.SaveUsageExportJobParams{
		ID:           job.ID(),
		WorkspaceID:  job.WorkspaceID(),
		Kind:         string(job.Kind()),
		Status:       string(job.Status()),
		Format:       string(job.Format()),
		QueryJson:    job.QueryJSON(),
		Error:        job.ErrorMessage(),
		Attempts:     int64(job.Attempts()),
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
	row, err := queriesFor(ctx, r.queries).FindUsageExportJob(ctx, sqlitedb.FindUsageExportJobParams{
		WorkspaceID: workspaceID,
		ID:          id,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainusage.ExportJob{}, domain.ErrNotFound
		}
		return domainusage.ExportJob{}, err
	}

	return exportJobFromFields(row.ID, row.WorkspaceID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, int(row.Attempts), row.LockedUntil, row.ArtifactPath, row.ArtifactSize, row.CreatedAt, row.UpdatedAt, row.CompletedAt)
}

func (r *UsageRepository) FindExportJobs(ctx context.Context, query domainusage.RunQuery) ([]domainusage.ExportJob, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	cursorCreatedAt, cursorID := runCursorValues(query)
	rows, err := queriesFor(ctx, r.queries).ListUsageExportJobs(ctx, sqlitedb.ListUsageExportJobsParams{
		WorkspaceID:     workspaceID,
		CursorCreatedAt: cursorCreatedAt,
		CursorID:        cursorID,
		Limit:           int64(query.Limit()),
	})
	if err != nil {
		return nil, err
	}

	jobs := make([]domainusage.ExportJob, 0, len(rows))
	for _, row := range rows {
		job, err := exportJobFromFields(row.ID, row.WorkspaceID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, int(row.Attempts), row.LockedUntil, row.ArtifactPath, row.ArtifactSize, row.CreatedAt, row.UpdatedAt, row.CompletedAt)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (r *UsageRepository) ClaimExportJob(ctx context.Context, now time.Time, lockedUntil time.Time, maxAttempts int) (domainusage.ExportJob, error) {
	row, err := queriesFor(ctx, r.queries).ClaimUsageExportJob(ctx, sqlitedb.ClaimUsageExportJobParams{
		Now:         formatTime(now),
		LockedUntil: exportJobTimeValue(lockedUntil),
		MaxAttempts: int64(maxAttempts),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainusage.ExportJob{}, domain.ErrNotFound
		}
		return domainusage.ExportJob{}, err
	}

	return exportJobFromFields(row.ID, row.WorkspaceID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, int(row.Attempts), row.LockedUntil, row.ArtifactPath, row.ArtifactSize, row.CreatedAt, row.UpdatedAt, row.CompletedAt)
}

func (r *UsageRepository) CompleteExportJob(ctx context.Context, id string, artifactPath string, artifactSize int64, completedAt time.Time) (domainusage.ExportJob, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.ExportJob{}, err
	}
	row, err := queriesFor(ctx, r.queries).CompleteUsageExportJob(ctx, sqlitedb.CompleteUsageExportJobParams{
		ID:           id,
		WorkspaceID:  workspaceID,
		ArtifactPath: artifactPath,
		ArtifactSize: artifactSize,
		CompletedAt:  formatTime(completedAt),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainusage.ExportJob{}, domain.ErrNotFound
		}
		return domainusage.ExportJob{}, err
	}

	return exportJobFromFields(row.ID, row.WorkspaceID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, int(row.Attempts), row.LockedUntil, row.ArtifactPath, row.ArtifactSize, row.CreatedAt, row.UpdatedAt, row.CompletedAt)
}

func (r *UsageRepository) FailExportJob(ctx context.Context, id string, errorMessage string, failedAt time.Time) (domainusage.ExportJob, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.ExportJob{}, err
	}
	row, err := queriesFor(ctx, r.queries).FailUsageExportJob(ctx, sqlitedb.FailUsageExportJobParams{
		ID:          id,
		WorkspaceID: workspaceID,
		Error:       errorMessage,
		FailedAt:    formatTime(failedAt),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainusage.ExportJob{}, domain.ErrNotFound
		}
		return domainusage.ExportJob{}, err
	}

	return exportJobFromFields(row.ID, row.WorkspaceID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, int(row.Attempts), row.LockedUntil, row.ArtifactPath, row.ArtifactSize, row.CreatedAt, row.UpdatedAt, row.CompletedAt)
}

func (r *UsageRepository) CancelExportJob(ctx context.Context, id string, canceledAt time.Time) (domainusage.ExportJob, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.ExportJob{}, err
	}
	row, err := queriesFor(ctx, r.queries).CancelUsageExportJob(ctx, sqlitedb.CancelUsageExportJobParams{
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

	return exportJobFromFields(row.ID, row.WorkspaceID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, int(row.Attempts), row.LockedUntil, row.ArtifactPath, row.ArtifactSize, row.CreatedAt, row.UpdatedAt, row.CompletedAt)
}

func (r *UsageRepository) RetryExportJob(ctx context.Context, id string, retriedAt time.Time) (domainusage.ExportJob, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.ExportJob{}, err
	}
	row, err := queriesFor(ctx, r.queries).RetryUsageExportJob(ctx, sqlitedb.RetryUsageExportJobParams{
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

	return exportJobFromFields(row.ID, row.WorkspaceID, row.Kind, row.Status, row.Format, row.QueryJson, row.Error, int(row.Attempts), row.LockedUntil, row.ArtifactPath, row.ArtifactSize, row.CreatedAt, row.UpdatedAt, row.CompletedAt)
}

func (r *UsageRepository) findByIdempotencyKey(ctx context.Context, key string) (domainusage.Event, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.Event{}, err
	}
	event, err := queriesFor(ctx, r.queries).FindUsageEventByIdempotencyKey(ctx, sqlitedb.FindUsageEventByIdempotencyKeyParams{
		WorkspaceID:    workspaceID,
		IdempotencyKey: sql.NullString{String: key, Valid: true},
	})
	return eventFromFields(event.ID, event.IdempotencyKey, event.Subject, event.MeterName, event.Quantity, event.EventTime, event.ReceivedAt, event.Metadata, err)
}

func (r *UsageRepository) findByID(ctx context.Context, id string) (domainusage.Event, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainusage.Event{}, err
	}
	event, err := queriesFor(ctx, r.queries).FindUsageEventByID(ctx, sqlitedb.FindUsageEventByIDParams{
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
	response, err := queriesFor(ctx, r.queries).FindBulkUsageIngestion(ctx, sqlitedb.FindBulkUsageIngestionParams{
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

func ingestionRunFromFields(id string, kind string, accepted int64, duplicates int64, failed int64, createdAtText string) (domainusage.IngestionRun, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtText)
	if err != nil {
		return domainusage.IngestionRun{}, err
	}

	return domainusage.NewIngestionRun(id, domainusage.IngestionKind(kind), int(accepted), int(duplicates), int(failed), createdAt)
}

func exportJobFromFields(id string, workspaceID string, kind string, status string, format string, queryJSON string, errorMessage string, attempts int, lockedUntilText sql.NullString, artifactPath string, artifactSize int64, createdAtText string, updatedAtText string, completedAtText sql.NullString) (domainusage.ExportJob, error) {
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
		artifactPath,
		artifactSize,
		createdAt,
		updatedAt,
		completedAt,
	)
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
