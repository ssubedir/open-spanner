package memory

import (
	"context"
	"sort"
	"time"

	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type UsageRepository struct {
	store *Store
}

func NewUsageRepository(store *Store) *UsageRepository {
	return &UsageRepository{store: store}
}

func (r *UsageRepository) Save(ctx context.Context, event domainusage.Event) (domainusage.Event, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	saved, _ := r.saveLocked(event)
	return saved, nil
}

func (r *UsageRepository) SaveBulk(ctx context.Context, idempotencyKey string, events []domainusage.Event) (domainusage.BulkSaveResult, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	if idempotencyKey != "" {
		if existing, exists := r.store.bulkKeys[idempotencyKey]; exists {
			return existing, nil
		}
	}

	accepted := make([]domainusage.Event, 0, len(events))
	duplicates := []domainusage.Event{}
	for _, event := range events {
		savedEvent, duplicate := r.saveLocked(event)
		if duplicate {
			duplicates = append(duplicates, savedEvent)
			continue
		}
		accepted = append(accepted, savedEvent)
	}

	result := domainusage.NewBulkSaveResult(accepted, duplicates)
	if idempotencyKey != "" {
		r.store.bulkKeys[idempotencyKey] = result
	}

	return result, nil
}

func (r *UsageRepository) Query(ctx context.Context, query domainusage.Query) ([]domainusage.Bucket, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	events := make([]domainusage.Event, 0, len(r.store.events))
	for _, event := range r.store.events {
		if query.Filter().Matches(event) {
			events = append(events, event)
		}
	}

	return domainusage.AggregateEvents(query, events), nil
}

func (r *UsageRepository) FindEvents(ctx context.Context, query domainusage.EventQuery) (domainusage.EventPage, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	events := make([]domainusage.Event, 0, len(r.store.events))
	for _, event := range r.store.events {
		if query.Subject() != "" && event.Subject() != query.Subject() {
			continue
		}
		if query.MeterName() != "" && event.MeterName() != query.MeterName() {
			continue
		}
		if !query.From().IsZero() && event.EventTime().Before(query.From()) {
			continue
		}
		if !query.To().IsZero() && !event.EventTime().Before(query.To()) {
			continue
		}
		if !query.Filter().Matches(event) {
			continue
		}
		events = append(events, event)
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].EventTime().Equal(events[j].EventTime()) {
			return events[i].ID() > events[j].ID()
		}
		return events[i].EventTime().After(events[j].EventTime())
	})

	if !query.Cursor().IsZero() {
		paged := make([]domainusage.Event, 0, len(events))
		for _, event := range events {
			if event.EventTime().Before(query.Cursor().EventTime()) ||
				(event.EventTime().Equal(query.Cursor().EventTime()) && event.ID() < query.Cursor().ID()) {
				paged = append(paged, event)
			}
		}
		events = paged
	}

	return domainusage.NewEventPage(events, query.Limit()), nil
}

func (r *UsageRepository) CountEvents(ctx context.Context) (int, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	return len(r.store.events), nil
}

func (r *UsageRepository) FindMeterStats(ctx context.Context) ([]domainusage.MeterStats, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	type aggregate struct {
		count       int
		lastEventAt time.Time
	}
	aggregates := map[string]aggregate{}
	for _, event := range r.store.events {
		current := aggregates[event.MeterName()]
		current.count++
		if current.lastEventAt.IsZero() || event.EventTime().After(current.lastEventAt) {
			current.lastEventAt = event.EventTime()
		}
		aggregates[event.MeterName()] = current
	}

	stats := make([]domainusage.MeterStats, 0, len(aggregates))
	for meterName, aggregate := range aggregates {
		stats = append(stats, domainusage.NewMeterStats(meterName, aggregate.count, aggregate.lastEventAt))
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].MeterName() < stats[j].MeterName()
	})

	return stats, nil
}

func (r *UsageRepository) FindSubjectStats(ctx context.Context, query domainusage.SubjectStatsQuery) ([]domainusage.SubjectStats, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	type aggregate struct {
		count       int
		meters      map[string]struct{}
		lastEventAt time.Time
	}
	aggregates := map[string]aggregate{}
	for _, event := range r.store.events {
		current := aggregates[event.Subject()]
		if current.meters == nil {
			current.meters = map[string]struct{}{}
		}
		current.count++
		current.meters[event.MeterName()] = struct{}{}
		if current.lastEventAt.IsZero() || event.EventTime().After(current.lastEventAt) {
			current.lastEventAt = event.EventTime()
		}
		aggregates[event.Subject()] = current
	}

	stats := make([]domainusage.SubjectStats, 0, len(aggregates))
	for subject, aggregate := range aggregates {
		stats = append(stats, domainusage.NewSubjectStats(subject, aggregate.count, len(aggregate.meters), aggregate.lastEventAt))
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].LastEventAt().Equal(stats[j].LastEventAt()) {
			return stats[i].Subject() < stats[j].Subject()
		}
		return stats[i].LastEventAt().After(stats[j].LastEventAt())
	})

	if query.HasCursor() {
		paged := make([]domainusage.SubjectStats, 0, len(stats))
		for _, stat := range stats {
			if stat.LastEventAt().Before(query.LastEventAt()) ||
				(stat.LastEventAt().Equal(query.LastEventAt()) && stat.Subject() > query.Subject()) {
				paged = append(paged, stat)
			}
		}
		stats = paged
	}

	limit := query.Limit()
	if limit < len(stats) {
		return stats[:limit], nil
	}

	return stats, nil
}

func (r *UsageRepository) PruneEvents(ctx context.Context, query domainusage.PruneQuery) (int, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	kept := make([]domainusage.Event, 0, len(r.store.events))
	deleted := 0
	for _, event := range r.store.events {
		if event.MeterName() == query.MeterName() && event.EventTime().Before(query.Before()) {
			deleted++
			if event.IdempotencyKey() != "" {
				delete(r.store.idempotencyKeys, event.IdempotencyKey())
			}
			continue
		}
		kept = append(kept, event)
	}
	r.store.events = kept

	return deleted, nil
}

func (r *UsageRepository) CountPrunableEvents(ctx context.Context, query domainusage.PruneQuery) (int, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	count := 0
	for _, event := range r.store.events {
		if event.MeterName() == query.MeterName() && event.EventTime().Before(query.Before()) {
			count++
		}
	}

	return count, nil
}

func (r *UsageRepository) SavePruneRun(ctx context.Context, run domainusage.PruneRun) (domainusage.PruneRun, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	r.store.pruneRuns = append(r.store.pruneRuns, run)
	return run, nil
}

func (r *UsageRepository) FindPruneRuns(ctx context.Context, query domainusage.RunQuery) ([]domainusage.PruneRun, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	runs := make([]domainusage.PruneRun, len(r.store.pruneRuns))
	copy(runs, r.store.pruneRuns)
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].CreatedAt().Equal(runs[j].CreatedAt()) {
			return runs[i].ID() > runs[j].ID()
		}
		return runs[i].CreatedAt().After(runs[j].CreatedAt())
	})

	if query.HasCursor() {
		paged := make([]domainusage.PruneRun, 0, len(runs))
		for _, run := range runs {
			if run.CreatedAt().Before(query.CreatedAt()) ||
				(run.CreatedAt().Equal(query.CreatedAt()) && run.ID() < query.ID()) {
				paged = append(paged, run)
			}
		}
		runs = paged
	}

	limit := query.Limit()
	if limit < len(runs) {
		return runs[:limit], nil
	}

	return runs, nil
}

func (r *UsageRepository) CountPruneRuns(ctx context.Context) (int, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	return len(r.store.pruneRuns), nil
}

func (r *UsageRepository) SaveIngestionRun(ctx context.Context, run domainusage.IngestionRun) (domainusage.IngestionRun, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	r.store.ingestionRuns = append(r.store.ingestionRuns, run)
	return run, nil
}

func (r *UsageRepository) FindIngestionRuns(ctx context.Context, query domainusage.RunQuery) ([]domainusage.IngestionRun, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	runs := make([]domainusage.IngestionRun, len(r.store.ingestionRuns))
	copy(runs, r.store.ingestionRuns)
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].CreatedAt().Equal(runs[j].CreatedAt()) {
			return runs[i].ID() > runs[j].ID()
		}
		return runs[i].CreatedAt().After(runs[j].CreatedAt())
	})

	if query.HasCursor() {
		paged := make([]domainusage.IngestionRun, 0, len(runs))
		for _, run := range runs {
			if run.CreatedAt().Before(query.CreatedAt()) ||
				(run.CreatedAt().Equal(query.CreatedAt()) && run.ID() < query.ID()) {
				paged = append(paged, run)
			}
		}
		runs = paged
	}

	limit := query.Limit()
	if limit < len(runs) {
		return runs[:limit], nil
	}

	return runs, nil
}

func (r *UsageRepository) saveLocked(event domainusage.Event) (domainusage.Event, bool) {
	if event.IdempotencyKey() != "" {
		if existing, exists := r.store.idempotencyKeys[event.IdempotencyKey()]; exists {
			return existing, true
		}
		r.store.idempotencyKeys[event.IdempotencyKey()] = event
	}

	r.store.events = append(r.store.events, event)
	return event, false
}
