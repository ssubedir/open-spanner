package repositorytest

import (
	"context"
	"errors"
	"testing"
	"time"

	apptransaction "github.com/ssubedir/open-spanner/internal/metering/app/transaction"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type SetupFunc func(t *testing.T, ctx context.Context) (domainmeter.Repository, domainusage.Repository, apptransaction.Transactor)

func Run(t *testing.T, setup SetupFunc) {
	t.Helper()

	t.Run("meter crud", func(t *testing.T) {
		ctx := context.Background()
		meterRepo, _, _ := setup(t, ctx)

		meter := newMeter(t, "meter-1", "api_calls")
		if _, err := meterRepo.Save(ctx, meter); err != nil {
			t.Fatalf("save meter: %v", err)
		}
		if _, err := meterRepo.Save(ctx, meter.WithDescription("updated")); err != nil {
			t.Fatalf("update meter: %v", err)
		}
		updatedDefinition, err := domainmeter.NewWithDimensions(
			meter.ID(),
			meter.Name(),
			"updated definition",
			"request",
			domainmeter.AggregationCount,
			[]domainmeter.Dimension{newDimension(t, "plan", domainmeter.MetadataString, "Plan", "Billing plan", true, true)},
			365,
			meter.CreatedAt(),
		)
		if err != nil {
			t.Fatalf("new updated meter: %v", err)
		}
		if _, err := meterRepo.Save(ctx, updatedDefinition); err != nil {
			t.Fatalf("update meter definition: %v", err)
		}

		byID, err := meterRepo.Find(ctx, domainmeter.Query{ID: "meter-1"})
		if err != nil {
			t.Fatalf("find meter by id: %v", err)
		}
		if len(byID) != 1 || byID[0].Description() != "updated definition" || byID[0].Unit() != "request" || byID[0].Aggregation() != domainmeter.AggregationCount || byID[0].EventRetentionDays() != 365 || byID[0].MetadataSchema()["plan"] != domainmeter.MetadataString || len(byID[0].Dimensions()) != 1 || byID[0].Dimensions()[0].DisplayName() != "Plan" || !byID[0].Dimensions()[0].Deprecated() {
			t.Fatalf("meter by id = %#v", byID)
		}

		byName, err := meterRepo.Find(ctx, domainmeter.Query{Name: "api_calls"})
		if err != nil {
			t.Fatalf("find meter by name: %v", err)
		}
		if len(byName) != 1 || byName[0].ID() != "meter-1" {
			t.Fatalf("meter by name = %#v", byName)
		}

		count, err := meterRepo.Count(ctx)
		if err != nil {
			t.Fatalf("count meters: %v", err)
		}
		if count != 1 {
			t.Fatalf("meter count = %d, want 1", count)
		}

		if err := meterRepo.Delete(ctx, domainmeter.Query{ID: "meter-1"}); err != nil {
			t.Fatalf("delete meter: %v", err)
		}
		remaining, err := meterRepo.Find(ctx, domainmeter.Query{ID: "meter-1"})
		if err != nil {
			t.Fatalf("find deleted meter: %v", err)
		}
		if len(remaining) != 0 {
			t.Fatalf("deleted meter still found: %#v", remaining)
		}
	})

	t.Run("usage idempotency and aggregation", func(t *testing.T) {
		ctx := context.Background()
		meterRepo, usageRepo, _ := setup(t, ctx)
		saveMeter(t, ctx, meterRepo, "meter-1", "api_calls")

		eventTime := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
		first := newEvent(t, "event-1", "usage-1", "org_123", "api_calls", 2, eventTime, nil)
		saved, err := usageRepo.Save(ctx, first)
		if err != nil {
			t.Fatalf("save usage: %v", err)
		}

		duplicate := newEvent(t, "event-2", "usage-1", "org_123", "api_calls", 100, eventTime.Add(time.Hour), nil)
		replayed, err := usageRepo.Save(ctx, duplicate)
		if err != nil {
			t.Fatalf("save duplicate usage: %v", err)
		}
		if replayed.ID() != saved.ID() || replayed.Quantity() != saved.Quantity() {
			t.Fatalf("replayed event = %#v, want saved %#v", replayed, saved)
		}

		if _, err := usageRepo.Save(ctx, newEvent(t, "event-3", "", "org_123", "api_calls", 3, eventTime.Add(2*time.Hour), nil)); err != nil {
			t.Fatalf("save later usage: %v", err)
		}

		query := newQuery(t, "org_123", "api_calls", domainusage.BucketDay, domainmeter.AggregationSum, domainusage.EmptyFilter(), "")
		buckets, err := usageRepo.Query(ctx, query)
		if err != nil {
			t.Fatalf("query usage: %v", err)
		}
		if len(buckets) != 1 || buckets[0].Quantity() != 5 {
			t.Fatalf("usage buckets = %#v, want one bucket with quantity 5", buckets)
		}
	})

	t.Run("event id conflicts", func(t *testing.T) {
		ctx := context.Background()
		meterRepo, usageRepo, _ := setup(t, ctx)
		saveMeter(t, ctx, meterRepo, "meter-1", "events")

		first := newEvent(t, "event-1", "", "org_123", "events", 2, time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC), nil)
		if _, err := usageRepo.Save(ctx, first); err != nil {
			t.Fatalf("save event: %v", err)
		}

		_, err := usageRepo.Save(ctx, newEvent(t, "event-1", "", "org_123", "events", 9, time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC), nil))
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("duplicate event id error = %v, want ErrConflict", err)
		}

		_, err = usageRepo.SaveBulk(ctx, "", []domainusage.Event{
			newEvent(t, "event-2", "", "org_123", "events", 3, time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC), nil),
			newEvent(t, "event-2", "", "org_123", "events", 4, time.Date(2026, 6, 8, 13, 0, 0, 0, time.UTC), nil),
		})
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("bulk duplicate event id error = %v, want ErrConflict", err)
		}

		query := newQuery(t, "org_123", "events", domainusage.BucketDay, domainmeter.AggregationSum, domainusage.EmptyFilter(), "")
		buckets, err := usageRepo.Query(ctx, query)
		if err != nil {
			t.Fatalf("query after bulk conflict: %v", err)
		}
		if totalQuantity(buckets) != 2 {
			t.Fatalf("buckets after rollback = %#v, want only first event", buckets)
		}
	})

	t.Run("bulk replay and duplicate events", func(t *testing.T) {
		ctx := context.Background()
		meterRepo, usageRepo, _ := setup(t, ctx)
		saveMeter(t, ctx, meterRepo, "meter-1", "tokens")

		events := []domainusage.Event{
			newEvent(t, "event-1", "usage-1", "org_123", "tokens", 2, time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC), nil),
			newEvent(t, "event-2", "usage-2", "org_123", "tokens", 3, time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC), nil),
		}
		first, err := usageRepo.SaveBulk(ctx, "batch-1", events)
		if err != nil {
			t.Fatalf("save bulk: %v", err)
		}

		replay, err := usageRepo.SaveBulk(ctx, "batch-1", []domainusage.Event{
			newEvent(t, "event-3", "usage-3", "org_123", "tokens", 100, time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC), nil),
		})
		if err != nil {
			t.Fatalf("replay bulk: %v", err)
		}
		if len(replay.Accepted()) != len(first.Accepted()) || len(replay.Duplicates()) != 0 {
			t.Fatalf("replayed bulk = %#v, want original accepted result %#v", replay, first)
		}

		second, err := usageRepo.SaveBulk(ctx, "", []domainusage.Event{
			newEvent(t, "event-4", "usage-1", "org_123", "tokens", 100, time.Date(2026, 6, 8, 13, 0, 0, 0, time.UTC), nil),
			newEvent(t, "event-5", "usage-4", "org_123", "tokens", 4, time.Date(2026, 6, 8, 14, 0, 0, 0, time.UTC), nil),
		})
		if err != nil {
			t.Fatalf("save duplicate bulk: %v", err)
		}
		if len(second.Accepted()) != 1 || len(second.Duplicates()) != 1 {
			t.Fatalf("duplicate bulk result = %#v", second)
		}
	})

	t.Run("advanced filters and event pagination", func(t *testing.T) {
		ctx := context.Background()
		meterRepo, usageRepo, _ := setup(t, ctx)
		saveMeter(t, ctx, meterRepo, "meter-1", "filtered")

		events := []domainusage.Event{
			newEvent(t, "event-1", "", "org_123", "filtered", 2, time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC), map[string]any{"region": "us-east-1"}),
			newEvent(t, "event-2", "", "org_123", "filtered", 3, time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC), map[string]any{"region": "us-west-2"}),
			newEvent(t, "event-3", "", "org_123", "filtered", 5, time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC), map[string]any{"region": "us-east-1"}),
		}
		for _, event := range events {
			if _, err := usageRepo.Save(ctx, event); err != nil {
				t.Fatalf("save usage %s: %v", event.ID(), err)
			}
		}

		filter, err := domainusage.NewFilterCondition("metadata.region", domainusage.FilterOpEqual, "us-east-1", true)
		if err != nil {
			t.Fatalf("new filter: %v", err)
		}
		query := newQuery(t, "org_123", "filtered", domainusage.BucketDay, domainmeter.AggregationSum, filter, "region")
		buckets, err := usageRepo.Query(ctx, query)
		if err != nil {
			t.Fatalf("query filtered usage: %v", err)
		}
		if len(buckets) != 1 || buckets[0].Quantity() != 7 || buckets[0].Group()["region"] != "us-east-1" {
			t.Fatalf("filtered buckets = %#v", buckets)
		}

		eventQuery, err := domainusage.NewFilteredEventQuery(
			"org_123",
			"filtered",
			time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
			1,
			domainusage.EventCursor{},
			filter,
		)
		if err != nil {
			t.Fatalf("new event query: %v", err)
		}
		page, err := usageRepo.FindEvents(ctx, eventQuery)
		if err != nil {
			t.Fatalf("find filtered events: %v", err)
		}
		if len(page.Events()) != 1 || page.NextCursor().IsZero() {
			t.Fatalf("first event page = %#v, want one item and next cursor", page)
		}
	})

	t.Run("usage groups by multiple metadata dimensions", func(t *testing.T) {
		ctx := context.Background()
		meterRepo, usageRepo, _ := setup(t, ctx)
		saveMeter(t, ctx, meterRepo, "meter-1", "dimensioned")

		events := []domainusage.Event{
			newEvent(t, "event-1", "", "org_123", "dimensioned", 2, time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC), map[string]any{"region": "us-east-1", "plan": "free"}),
			newEvent(t, "event-2", "", "org_123", "dimensioned", 3, time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC), map[string]any{"region": "us-east-1", "plan": "pro"}),
			newEvent(t, "event-3", "", "org_123", "dimensioned", 5, time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC), map[string]any{"region": "us-east-1", "plan": "free"}),
		}
		for _, event := range events {
			if _, err := usageRepo.Save(ctx, event); err != nil {
				t.Fatalf("save usage %s: %v", event.ID(), err)
			}
		}

		query := newGroupedQuery(t, "org_123", "dimensioned", domainusage.BucketDay, domainmeter.AggregationSum, domainusage.EmptyFilter(), []string{"region", "plan"})
		buckets, err := usageRepo.Query(ctx, query)
		if err != nil {
			t.Fatalf("query grouped usage: %v", err)
		}
		if len(buckets) != 2 {
			t.Fatalf("bucket count = %d, want 2: %#v", len(buckets), buckets)
		}
		if buckets[0].Group()["region"] != "us-east-1" || buckets[0].Group()["plan"] != "free" || buckets[0].Quantity() != 7 {
			t.Fatalf("first grouped bucket = %#v", buckets[0])
		}
		if buckets[1].Group()["region"] != "us-east-1" || buckets[1].Group()["plan"] != "pro" || buckets[1].Quantity() != 3 {
			t.Fatalf("second grouped bucket = %#v", buckets[1])
		}
	})

	t.Run("usage aggregates across subjects", func(t *testing.T) {
		ctx := context.Background()
		meterRepo, usageRepo, _ := setup(t, ctx)
		saveMeter(t, ctx, meterRepo, "meter-1", "accounts")

		events := []domainusage.Event{
			newEvent(t, "event-1", "", "org_123", "accounts", 2, time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC), nil),
			newEvent(t, "event-2", "", "org_123", "accounts", 3, time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC), nil),
			newEvent(t, "event-3", "", "org_456", "accounts", 5, time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC), nil),
		}
		for _, event := range events {
			if _, err := usageRepo.Save(ctx, event); err != nil {
				t.Fatalf("save usage %s: %v", event.ID(), err)
			}
		}

		query := newQuery(t, "", "accounts", domainusage.BucketDay, domainmeter.AggregationSum, domainusage.EmptyFilter(), "")
		buckets, err := usageRepo.Query(ctx, query)
		if err != nil {
			t.Fatalf("query all subject usage: %v", err)
		}
		if len(buckets) != 1 || buckets[0].Subject() != "" || buckets[0].Quantity() != 10 {
			t.Fatalf("all subject buckets = %#v, want one unscoped bucket with quantity 10", buckets)
		}

		groupedQuery := newGroupedQuery(t, "", "accounts", domainusage.BucketDay, domainmeter.AggregationSum, domainusage.EmptyFilter(), []string{domainusage.GroupBySubject})
		groupedBuckets, err := usageRepo.Query(ctx, groupedQuery)
		if err != nil {
			t.Fatalf("query grouped subject usage: %v", err)
		}
		if len(groupedBuckets) != 2 {
			t.Fatalf("grouped bucket count = %d, want 2: %#v", len(groupedBuckets), groupedBuckets)
		}
		if groupedBuckets[0].Subject() != "org_123" || groupedBuckets[0].Group()[domainusage.GroupBySubject] != "org_123" || groupedBuckets[0].Quantity() != 5 {
			t.Fatalf("first grouped subject bucket = %#v", groupedBuckets[0])
		}
		if groupedBuckets[1].Subject() != "org_456" || groupedBuckets[1].Group()[domainusage.GroupBySubject] != "org_456" || groupedBuckets[1].Quantity() != 5 {
			t.Fatalf("second grouped subject bucket = %#v", groupedBuckets[1])
		}
	})

	t.Run("usage discovers dimension values", func(t *testing.T) {
		ctx := context.Background()
		meterRepo, usageRepo, _ := setup(t, ctx)
		saveMeter(t, ctx, meterRepo, "meter-1", "discoverable")

		events := []domainusage.Event{
			newEvent(t, "event-1", "", "org_123", "discoverable", 2, time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC), map[string]any{"region": "us-east-1"}),
			newEvent(t, "event-2", "", "org_123", "discoverable", 3, time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC), map[string]any{"region": "us-west-2"}),
			newEvent(t, "event-3", "", "org_123", "discoverable", 5, time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC), map[string]any{"region": "us-east-1"}),
			newEvent(t, "event-4", "", "org_456", "discoverable", 7, time.Date(2026, 6, 8, 13, 0, 0, 0, time.UTC), map[string]any{"region": "us-central-1"}),
			newEvent(t, "event-5", "", "org_123", "discoverable", 11, time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC), map[string]any{"region": "eu-west-1"}),
		}
		for _, event := range events {
			if _, err := usageRepo.Save(ctx, event); err != nil {
				t.Fatalf("save usage %s: %v", event.ID(), err)
			}
		}

		query, err := domainusage.NewDimensionValueQuery(
			"discoverable",
			"region",
			"org_123",
			time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
			10,
		)
		if err != nil {
			t.Fatalf("new dimension value query: %v", err)
		}

		values, err := usageRepo.FindDimensionValues(ctx, query)
		if err != nil {
			t.Fatalf("find dimension values: %v", err)
		}
		if len(values) != 2 {
			t.Fatalf("dimension values = %#v, want two values", values)
		}
		if values[0].Field() != "region" || values[0].Value() != "us-east-1" || values[0].UsageEvents() != 2 {
			t.Fatalf("first dimension value = %#v", values[0])
		}
		if values[1].Value() != "us-west-2" || values[1].UsageEvents() != 1 {
			t.Fatalf("second dimension value = %#v", values[1])
		}
	})

	t.Run("usage finds breakdowns", func(t *testing.T) {
		ctx := context.Background()
		meterRepo, usageRepo, _ := setup(t, ctx)
		saveMeter(t, ctx, meterRepo, "meter-1", "breakdowns")

		events := []domainusage.Event{
			newEvent(t, "event-1", "", "org_123", "breakdowns", 2, time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC), map[string]any{"endpoint": "/orders"}),
			newEvent(t, "event-2", "", "org_123", "breakdowns", 3, time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC), map[string]any{"endpoint": "/users"}),
			newEvent(t, "event-3", "", "org_456", "breakdowns", 7, time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC), map[string]any{"endpoint": "/orders"}),
			newEvent(t, "event-4", "", "org_789", "breakdowns", 1, time.Date(2026, 6, 8, 13, 0, 0, 0, time.UTC), map[string]any{"endpoint": "/users"}),
		}
		for _, event := range events {
			if _, err := usageRepo.Save(ctx, event); err != nil {
				t.Fatalf("save usage %s: %v", event.ID(), err)
			}
		}

		subjectQuery := newBreakdownQuery(t, "", "breakdowns", domainusage.GroupBySubject, domainmeter.AggregationSum, domainusage.EmptyFilter())
		subjects, err := usageRepo.FindBreakdown(ctx, subjectQuery)
		if err != nil {
			t.Fatalf("find subject breakdown: %v", err)
		}
		if len(subjects) != 3 {
			t.Fatalf("subject breakdowns = %#v, want three items", subjects)
		}
		if subjects[0].Value() != "org_456" || subjects[0].Quantity() != 7 || subjects[0].UsageEvents() != 1 {
			t.Fatalf("first subject breakdown = %#v", subjects[0])
		}
		if subjects[1].Value() != "org_123" || subjects[1].Quantity() != 5 || subjects[1].UsageEvents() != 2 {
			t.Fatalf("second subject breakdown = %#v", subjects[1])
		}
		if subjects[2].Value() != "org_789" || subjects[2].Quantity() != 1 || subjects[2].UsageEvents() != 1 {
			t.Fatalf("third subject breakdown = %#v", subjects[2])
		}

		endpointQuery := newBreakdownQuery(t, "", "breakdowns", "endpoint", domainmeter.AggregationSum, domainusage.EmptyFilter())
		endpoints, err := usageRepo.FindBreakdown(ctx, endpointQuery)
		if err != nil {
			t.Fatalf("find endpoint breakdown: %v", err)
		}
		if len(endpoints) != 2 {
			t.Fatalf("endpoint breakdowns = %#v, want two items", endpoints)
		}
		if endpoints[0].Value() != "/orders" || endpoints[0].Quantity() != 9 || endpoints[0].UsageEvents() != 2 {
			t.Fatalf("first endpoint breakdown = %#v", endpoints[0])
		}
		if endpoints[1].Value() != "/users" || endpoints[1].Quantity() != 4 || endpoints[1].UsageEvents() != 2 {
			t.Fatalf("second endpoint breakdown = %#v", endpoints[1])
		}

		scopedQuery := newBreakdownQuery(t, "org_123", "breakdowns", "endpoint", domainmeter.AggregationSum, domainusage.EmptyFilter())
		scoped, err := usageRepo.FindBreakdown(ctx, scopedQuery)
		if err != nil {
			t.Fatalf("find scoped endpoint breakdown: %v", err)
		}
		if len(scoped) != 2 || scoped[0].Value() != "/users" || scoped[0].Quantity() != 3 || scoped[1].Value() != "/orders" || scoped[1].Quantity() != 2 {
			t.Fatalf("scoped endpoint breakdowns = %#v", scoped)
		}
	})

	t.Run("prune transaction rollback", func(t *testing.T) {
		ctx := context.Background()
		meterRepo, usageRepo, transactor := setup(t, ctx)
		saveMeter(t, ctx, meterRepo, "meter-1", "retained")

		before := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
		if _, err := usageRepo.Save(ctx, newEvent(t, "old-event", "", "org_123", "retained", 1, before.Add(-time.Hour), nil)); err != nil {
			t.Fatalf("save old event: %v", err)
		}
		if _, err := usageRepo.Save(ctx, newEvent(t, "new-event", "", "org_123", "retained", 2, before.Add(time.Hour), nil)); err != nil {
			t.Fatalf("save new event: %v", err)
		}

		pruneQuery, err := domainusage.NewPruneQuery("retained", before)
		if err != nil {
			t.Fatalf("new prune query: %v", err)
		}
		rollbackErr := errors.New("rollback prune")
		err = transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
			deleted, err := usageRepo.PruneEvents(txCtx, pruneQuery)
			if err != nil {
				return err
			}
			if deleted != 1 {
				t.Fatalf("deleted = %d, want 1", deleted)
			}
			return rollbackErr
		})
		if !errors.Is(err, rollbackErr) {
			t.Fatalf("transaction error = %v, want rollback error", err)
		}

		query := newQuery(t, "org_123", "retained", domainusage.BucketDay, domainmeter.AggregationSum, domainusage.EmptyFilter(), "")
		buckets, err := usageRepo.Query(ctx, query)
		if err != nil {
			t.Fatalf("query after rollback: %v", err)
		}
		if totalQuantity(buckets) != 3 {
			t.Fatalf("rollback buckets = %#v, want both events still present", buckets)
		}

		var run domainusage.PruneRun
		err = transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
			deleted, err := usageRepo.PruneEvents(txCtx, pruneQuery)
			if err != nil {
				return err
			}
			runMeter, err := domainusage.NewPruneRunMeter("retained", before, deleted)
			if err != nil {
				return err
			}
			run, err = domainusage.NewPruneRun("prune-1", false, deleted, []domainusage.PruneRunMeter{runMeter}, before)
			if err != nil {
				return err
			}
			run, err = usageRepo.SavePruneRun(txCtx, run)
			return err
		})
		if err != nil {
			t.Fatalf("commit prune transaction: %v", err)
		}
		if run.Deleted() != 1 {
			t.Fatalf("prune run deleted = %d, want 1", run.Deleted())
		}

		runs, err := usageRepo.FindPruneRuns(ctx, domainusage.NewRunQuery(10, time.Time{}, ""))
		if err != nil {
			t.Fatalf("find prune runs: %v", err)
		}
		if len(runs) != 1 || runs[0].ID() != "prune-1" {
			t.Fatalf("prune runs = %#v", runs)
		}
	})
}

func totalQuantity(buckets []domainusage.Bucket) float64 {
	total := 0.0
	for _, bucket := range buckets {
		total += bucket.Quantity()
	}
	return total
}

func saveMeter(t *testing.T, ctx context.Context, repo domainmeter.Repository, id string, name string) {
	t.Helper()
	if _, err := repo.Save(ctx, newMeter(t, id, name)); err != nil {
		t.Fatalf("save meter %s: %v", name, err)
	}
}

func newMeter(t *testing.T, id string, name string) domainmeter.Meter {
	t.Helper()
	meter, err := domainmeter.New(id, name, "contract meter", "count", domainmeter.AggregationSum, map[string]domainmeter.MetadataType{}, 0, time.Date(2026, 6, 8, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("new meter: %v", err)
	}
	return meter
}

func newDimension(t *testing.T, name string, metadataType domainmeter.MetadataType, displayName string, description string, required bool, deprecated ...bool) domainmeter.Dimension {
	t.Helper()
	dimension, err := domainmeter.NewDimension(name, metadataType, displayName, description, required, deprecated...)
	if err != nil {
		t.Fatalf("new dimension: %v", err)
	}
	return dimension
}

func newEvent(t *testing.T, id string, idempotencyKey string, subject string, meterName string, quantity float64, eventTime time.Time, metadata map[string]any) domainusage.Event {
	t.Helper()
	if metadata == nil {
		metadata = map[string]any{}
	}
	if _, exists := metadata["source"]; !exists {
		metadata["source"] = "contract"
	}
	event, err := domainusage.NewEvent(id, idempotencyKey, subject, meterName, quantity, eventTime, eventTime.Add(time.Second), metadata)
	if err != nil {
		t.Fatalf("new event: %v", err)
	}
	return event
}

func newQuery(t *testing.T, subject string, meterName string, bucketSize domainusage.BucketSize, aggregation domainmeter.Aggregation, filter domainusage.Filter, groupBy string) domainusage.Query {
	t.Helper()
	query, err := domainusage.NewFilteredQuery(
		subject,
		meterName,
		time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		bucketSize,
		aggregation,
		nil,
		groupBy,
		0,
		filter,
	)
	if err != nil {
		t.Fatalf("new query: %v", err)
	}
	return query
}

func newGroupedQuery(t *testing.T, subject string, meterName string, bucketSize domainusage.BucketSize, aggregation domainmeter.Aggregation, filter domainusage.Filter, groupBy []string) domainusage.Query {
	t.Helper()
	query, err := domainusage.NewGroupedFilteredQuery(
		subject,
		meterName,
		time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		bucketSize,
		aggregation,
		nil,
		groupBy,
		0,
		filter,
	)
	if err != nil {
		t.Fatalf("new grouped query: %v", err)
	}
	return query
}

func newBreakdownQuery(t *testing.T, subject string, meterName string, field string, aggregation domainmeter.Aggregation, filter domainusage.Filter) domainusage.BreakdownQuery {
	t.Helper()
	query, err := domainusage.NewBreakdownQuery(
		meterName,
		field,
		subject,
		time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		aggregation,
		0,
		filter,
	)
	if err != nil {
		t.Fatalf("new breakdown query: %v", err)
	}
	return query
}
