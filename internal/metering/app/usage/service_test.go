package usage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ssubedir/open-spanner/internal/config"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

func TestServiceCreateAndList(t *testing.T) {
	ctx := context.Background()
	service := newTestService(t, ctx)
	eventTime := time.Date(2026, 6, 8, 14, 30, 0, 0, time.UTC)

	created, err := service.Create(ctx, CreateCommand{
		IdempotencyKey: "usage-1",
		Subject:        "org_123",
		MeterName:      "api_calls",
		Quantity:       4,
		EventTime:      eventTime,
	})
	if err != nil {
		t.Fatalf("create usage: %v", err)
	}
	if created.ID == "" || created.MeterName != "api_calls" || created.Quantity != 4 {
		t.Fatalf("created usage = %#v", created)
	}

	buckets, err := service.List(ctx, ListQuery{
		Subject:    "org_123",
		MeterName:  "api_calls",
		From:       time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		BucketSize: domainusage.BucketDay,
	})
	if err != nil {
		t.Fatalf("list usage: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Quantity != 4 {
		t.Fatalf("usage buckets = %#v", buckets)
	}
}

func TestServiceCreateDefaultsEventTime(t *testing.T) {
	ctx := context.Background()
	service := newTestService(t, ctx)

	created, err := service.Create(ctx, CreateCommand{
		Subject:   "org_123",
		MeterName: "api_calls",
		Quantity:  1,
	})
	if err != nil {
		t.Fatalf("create usage: %v", err)
	}
	if created.EventTime.IsZero() {
		t.Fatal("event time was not defaulted")
	}
}

func TestServiceCreateBulk(t *testing.T) {
	ctx := context.Background()
	service := newTestService(t, ctx)

	created, err := service.CreateBulk(ctx, "", []CreateCommand{
		{
			IdempotencyKey: "usage-1",
			Subject:        "org_123",
			MeterName:      "api_calls",
			Quantity:       2,
			EventTime:      time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC),
		},
		{
			IdempotencyKey: "usage-2",
			Subject:        "org_123",
			MeterName:      "api_calls",
			Quantity:       3,
			EventTime:      time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("create bulk usage: %v", err)
	}
	if len(created.Accepted) != 2 || len(created.Duplicates) != 0 {
		t.Fatalf("created bulk usage = %#v", created)
	}

	buckets, err := service.List(ctx, ListQuery{
		Subject:    "org_123",
		MeterName:  "api_calls",
		From:       time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		BucketSize: domainusage.BucketDay,
	})
	if err != nil {
		t.Fatalf("list usage: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Quantity != 5 {
		t.Fatalf("usage buckets = %#v, want quantity 5", buckets)
	}
}

func TestServiceCreateBulkRejectsEmptyBatch(t *testing.T) {
	ctx := context.Background()
	service := newTestService(t, ctx)

	_, err := service.CreateBulk(ctx, "", nil)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("empty bulk error = %v, want ErrInvalidInput", err)
	}
}

func TestServiceCreateBulkRejectsOverLimitBatch(t *testing.T) {
	ctx := context.Background()
	service := newTestService(t, ctx)
	commands := make([]CreateCommand, MaxBulkEvents+1)
	for i := range commands {
		commands[i] = CreateCommand{
			Subject:   "org_123",
			MeterName: "api_calls",
			Quantity:  1,
		}
	}

	_, err := service.CreateBulk(ctx, "", commands)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("over-limit bulk error = %v, want ErrInvalidInput", err)
	}
}

func TestServiceCreateBulkReplaysIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	service := newTestService(t, ctx)
	commands := []CreateCommand{
		{
			IdempotencyKey: "usage-1",
			Subject:        "org_123",
			MeterName:      "api_calls",
			Quantity:       2,
			EventTime:      time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC),
		},
		{
			IdempotencyKey: "usage-2",
			Subject:        "org_123",
			MeterName:      "api_calls",
			Quantity:       3,
			EventTime:      time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC),
		},
	}

	first, err := service.CreateBulk(ctx, "batch-1", commands)
	if err != nil {
		t.Fatalf("create first bulk usage: %v", err)
	}
	second, err := service.CreateBulk(ctx, "batch-1", []CreateCommand{
		{
			IdempotencyKey: "usage-3",
			Subject:        "org_123",
			MeterName:      "api_calls",
			Quantity:       100,
			EventTime:      time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("replay bulk usage: %v", err)
	}
	if len(second.Accepted) != len(first.Accepted) || second.Accepted[0].ID != first.Accepted[0].ID || second.Accepted[1].ID != first.Accepted[1].ID || len(second.Duplicates) != 0 {
		t.Fatalf("replayed bulk = %#v, want original %#v", second, first)
	}

	buckets, err := service.List(ctx, ListQuery{
		Subject:    "org_123",
		MeterName:  "api_calls",
		From:       time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		BucketSize: domainusage.BucketDay,
	})
	if err != nil {
		t.Fatalf("list usage: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Quantity != 5 {
		t.Fatalf("usage buckets = %#v, want quantity 5", buckets)
	}
}

func TestServiceCreateBulkReportsDuplicateEventKeys(t *testing.T) {
	ctx := context.Background()
	service := newTestService(t, ctx)

	first, err := service.CreateBulk(ctx, "", []CreateCommand{
		{
			IdempotencyKey: "usage-1",
			Subject:        "org_123",
			MeterName:      "api_calls",
			Quantity:       2,
			EventTime:      time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("create first bulk usage: %v", err)
	}

	second, err := service.CreateBulk(ctx, "", []CreateCommand{
		{
			IdempotencyKey: "usage-1",
			Subject:        "org_123",
			MeterName:      "api_calls",
			Quantity:       100,
			EventTime:      time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC),
		},
		{
			IdempotencyKey: "usage-2",
			Subject:        "org_123",
			MeterName:      "api_calls",
			Quantity:       3,
			EventTime:      time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("create duplicate bulk usage: %v", err)
	}
	if len(second.Accepted) != 1 || len(second.Duplicates) != 1 {
		t.Fatalf("second bulk result = %#v", second)
	}
	if second.Duplicates[0].ID != first.Accepted[0].ID || second.Duplicates[0].Quantity != 2 {
		t.Fatalf("duplicate event = %#v, want original %#v", second.Duplicates[0], first.Accepted[0])
	}

	buckets, err := service.List(ctx, ListQuery{
		Subject:    "org_123",
		MeterName:  "api_calls",
		From:       time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		BucketSize: domainusage.BucketDay,
	})
	if err != nil {
		t.Fatalf("list usage: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Quantity != 5 {
		t.Fatalf("usage buckets = %#v, want quantity 5", buckets)
	}
}

func TestServiceCreateBulkReportsFailedItems(t *testing.T) {
	ctx := context.Background()
	service := newTestService(t, ctx)

	result, err := service.CreateBulk(ctx, "", []CreateCommand{
		{
			Index:     0,
			Subject:   "org_123",
			MeterName: "missing",
			Quantity:  2,
			EventTime: time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC),
		},
		{
			Index:     1,
			Subject:   "org_123",
			MeterName: "api_calls",
			Quantity:  3,
			EventTime: time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("create partial bulk usage: %v", err)
	}
	if len(result.Accepted) != 1 || len(result.Failed) != 1 || len(result.Duplicates) != 0 {
		t.Fatalf("bulk result = %#v", result)
	}
	if result.Failed[0].Index != 0 || result.Failed[0].Code != "not_found" {
		t.Fatalf("failed item = %#v", result.Failed[0])
	}

	buckets, err := service.List(ctx, ListQuery{
		Subject:    "org_123",
		MeterName:  "api_calls",
		From:       time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		BucketSize: domainusage.BucketDay,
	})
	if err != nil {
		t.Fatalf("list usage: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Quantity != 3 {
		t.Fatalf("usage buckets = %#v, want quantity 3", buckets)
	}
}

func TestServiceCreateBulkReturnsFailuresWhenAllItemsFail(t *testing.T) {
	ctx := context.Background()
	service := newTestService(t, ctx)

	result, err := service.CreateBulk(ctx, "", []CreateCommand{
		{
			Index:     0,
			Subject:   "org_123",
			MeterName: "missing",
			Quantity:  2,
			EventTime: time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("create failed bulk usage: %v", err)
	}
	if len(result.Accepted) != 0 || len(result.Duplicates) != 0 || len(result.Failed) != 1 {
		t.Fatalf("bulk result = %#v", result)
	}
}

func TestServiceListEvents(t *testing.T) {
	ctx := context.Background()
	service := newTestService(t, ctx)

	for i, quantity := range []float64{2, 3, 5} {
		_, err := service.Create(ctx, CreateCommand{
			Subject:   "org_123",
			MeterName: "api_calls",
			Quantity:  quantity,
			EventTime: time.Date(2026, 6, 8, 10+i, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("create usage %d: %v", i, err)
		}
	}

	page, err := service.ListEvents(ctx, EventListQuery{
		Subject:   "org_123",
		MeterName: "api_calls",
		From:      time.Date(2026, 6, 8, 10, 30, 0, 0, time.UTC),
		To:        time.Date(2026, 6, 8, 13, 0, 0, 0, time.UTC),
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("list usage events: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Quantity != 5 || page.NextCursor == "" {
		t.Fatalf("usage events page = %#v", page)
	}

	nextPage, err := service.ListEvents(ctx, EventListQuery{
		Subject:   "org_123",
		MeterName: "api_calls",
		From:      time.Date(2026, 6, 8, 10, 30, 0, 0, time.UTC),
		To:        time.Date(2026, 6, 8, 13, 0, 0, 0, time.UTC),
		Limit:     1,
		Cursor:    page.NextCursor,
	})
	if err != nil {
		t.Fatalf("list next usage events: %v", err)
	}
	if len(nextPage.Items) != 1 || nextPage.Items[0].Quantity != 3 || nextPage.NextCursor != "" {
		t.Fatalf("next usage events page = %#v", nextPage)
	}
}

func TestServicePruneEventsUsesMeterRetention(t *testing.T) {
	ctx := context.Background()
	store, meterRepo, usageRepo := newTestRepositories(t, ctx)

	meter, err := domainmeter.New(
		"meter-1",
		"api_calls",
		"API calls",
		"call",
		domainmeter.AggregationSum,
		map[string]domainmeter.MetadataType{},
		1,
		time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("new meter: %v", err)
	}
	if _, err := meterRepo.Save(ctx, meter); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	service := NewService(meterRepo, usageRepo, store).(*service)
	service.now = func() time.Time {
		return time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	}

	for _, event := range []CreateCommand{
		{Subject: "org_123", MeterName: "api_calls", Quantity: 1, EventTime: time.Date(2026, 6, 8, 23, 59, 59, 0, time.UTC)},
		{Subject: "org_123", MeterName: "api_calls", Quantity: 2, EventTime: time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)},
	} {
		if _, err := service.Create(ctx, event); err != nil {
			t.Fatalf("create usage: %v", err)
		}
	}

	dryRun, err := service.PruneEvents(ctx, PruneCommand{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run prune usage events: %v", err)
	}
	if dryRun.Deleted != 1 || !dryRun.DryRun {
		t.Fatalf("dry-run prune result = %#v", dryRun)
	}

	beforePrune, err := service.ListEvents(ctx, EventListQuery{MeterName: "api_calls", Limit: 10})
	if err != nil {
		t.Fatalf("list dry-run remaining events: %v", err)
	}
	if len(beforePrune.Items) != 2 {
		t.Fatalf("dry-run deleted events: %#v", beforePrune)
	}

	result, err := service.PruneEvents(ctx, PruneCommand{})
	if err != nil {
		t.Fatalf("prune usage events: %v", err)
	}
	if result.ID == "" || result.Deleted != 1 || result.DryRun || len(result.Meters) != 1 || result.Meters[0].Deleted != 1 || result.CreatedAt.IsZero() {
		t.Fatalf("prune result = %#v", result)
	}

	runs, err := service.ListPruneRuns(ctx, PruneRunListQuery{Limit: 10})
	if err != nil {
		t.Fatalf("list prune runs: %v", err)
	}
	if len(runs.Items) != 2 || pruneRunCountByMode(runs.Items, true) != 1 || pruneRunCountByMode(runs.Items, false) != 1 {
		t.Fatalf("prune runs = %#v", runs)
	}

	remaining, err := service.ListEvents(ctx, EventListQuery{MeterName: "api_calls", Limit: 10})
	if err != nil {
		t.Fatalf("list remaining events: %v", err)
	}
	if len(remaining.Items) != 1 || remaining.Items[0].Quantity != 2 {
		t.Fatalf("remaining events = %#v", remaining)
	}
}

func pruneRunCountByMode(runs []PruneResult, dryRun bool) int {
	count := 0
	for _, run := range runs {
		if run.DryRun == dryRun {
			count++
		}
	}
	return count
}

func TestServiceListUsesMeterAggregation(t *testing.T) {
	ctx := context.Background()
	store, meterRepo, usageRepo := newTestRepositories(t, ctx)

	meter, err := domainmeter.New(
		"meter-1",
		"latency_ms",
		"Request latency",
		"ms",
		domainmeter.AggregationAverage,
		map[string]domainmeter.MetadataType{},
		0,
		time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("new meter: %v", err)
	}
	if _, err := meterRepo.Save(ctx, meter); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	service := NewService(meterRepo, usageRepo, store)
	for i, quantity := range []float64{100, 300, 500} {
		_, err := service.Create(ctx, CreateCommand{
			Subject:   "org_123",
			MeterName: "latency_ms",
			Quantity:  quantity,
			EventTime: time.Date(2026, 6, 8, 14, i, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("create usage %d: %v", i, err)
		}
	}

	buckets, err := service.List(ctx, ListQuery{
		Subject:    "org_123",
		MeterName:  "latency_ms",
		From:       time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		BucketSize: domainusage.BucketDay,
	})
	if err != nil {
		t.Fatalf("list usage: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Quantity != 300 {
		t.Fatalf("usage buckets = %#v, want avg quantity 300", buckets)
	}
}

func TestServiceCreateRejectsMetadataOutsideMeterSchema(t *testing.T) {
	ctx := context.Background()
	service := newTestService(t, ctx)

	_, err := service.Create(ctx, CreateCommand{
		Subject:   "org_123",
		MeterName: "api_calls",
		Quantity:  1,
		Metadata:  map[string]any{"region": "us-east-1"},
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("metadata schema error = %v, want ErrInvalidInput", err)
	}
}

func TestServiceCreateAcceptsMetadataMatchingMeterSchema(t *testing.T) {
	ctx := context.Background()
	store, meterRepo, usageRepo := newTestRepositories(t, ctx)

	meter, err := domainmeter.New(
		"meter-1",
		"api_calls",
		"API calls",
		"call",
		domainmeter.AggregationSum,
		map[string]domainmeter.MetadataType{
			"region": domainmeter.MetadataString,
			"retry":  domainmeter.MetadataBoolean,
		},
		0,
		time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("new meter: %v", err)
	}
	if _, err := meterRepo.Save(ctx, meter); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	service := NewService(meterRepo, usageRepo, store)
	created, err := service.Create(ctx, CreateCommand{
		Subject:   "org_123",
		MeterName: "api_calls",
		Quantity:  1,
		Metadata:  map[string]any{"region": "us-east-1", "retry": false},
	})
	if err != nil {
		t.Fatalf("create usage: %v", err)
	}
	if created.Metadata["region"] != "us-east-1" {
		t.Fatalf("created metadata = %#v", created.Metadata)
	}
}

func TestServiceListGroupsByMultipleMetadataFields(t *testing.T) {
	ctx := context.Background()
	store, meterRepo, usageRepo := newTestRepositories(t, ctx)

	meter, err := domainmeter.New(
		"meter-1",
		"api_calls",
		"API calls",
		"call",
		domainmeter.AggregationSum,
		map[string]domainmeter.MetadataType{
			"region": domainmeter.MetadataString,
			"plan":   domainmeter.MetadataString,
		},
		0,
		time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("new meter: %v", err)
	}
	if _, err := meterRepo.Save(ctx, meter); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	service := NewService(meterRepo, usageRepo, store)
	for _, event := range []CreateCommand{
		{Subject: "org_123", MeterName: "api_calls", Quantity: 2, EventTime: time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC), Metadata: map[string]any{"region": "us-east-1", "plan": "free"}},
		{Subject: "org_123", MeterName: "api_calls", Quantity: 3, EventTime: time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC), Metadata: map[string]any{"region": "us-east-1", "plan": "pro"}},
		{Subject: "org_123", MeterName: "api_calls", Quantity: 5, EventTime: time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC), Metadata: map[string]any{"region": "us-east-1", "plan": "free"}},
	} {
		if _, err := service.Create(ctx, event); err != nil {
			t.Fatalf("create usage: %v", err)
		}
	}

	buckets, err := service.List(ctx, ListQuery{
		Subject:    "org_123",
		MeterName:  "api_calls",
		From:       time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		BucketSize: domainusage.BucketDay,
		GroupBy:    []string{"region", "plan"},
	})
	if err != nil {
		t.Fatalf("list usage: %v", err)
	}
	if len(buckets) != 2 {
		t.Fatalf("bucket count = %d, want 2: %#v", len(buckets), buckets)
	}
	if buckets[0].Group["region"] != "us-east-1" || buckets[0].Group["plan"] != "free" || buckets[0].Quantity != 7 {
		t.Fatalf("first grouped bucket = %#v", buckets[0])
	}
	if buckets[1].Group["region"] != "us-east-1" || buckets[1].Group["plan"] != "pro" || buckets[1].Quantity != 3 {
		t.Fatalf("second grouped bucket = %#v", buckets[1])
	}
}

func TestServiceListAggregatesAcrossSubjects(t *testing.T) {
	ctx := context.Background()
	service := newTestService(t, ctx)

	for _, event := range []CreateCommand{
		{Subject: "org_123", MeterName: "api_calls", Quantity: 2, EventTime: time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)},
		{Subject: "org_123", MeterName: "api_calls", Quantity: 3, EventTime: time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC)},
		{Subject: "org_456", MeterName: "api_calls", Quantity: 5, EventTime: time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)},
	} {
		if _, err := service.Create(ctx, event); err != nil {
			t.Fatalf("create usage: %v", err)
		}
	}

	buckets, err := service.List(ctx, ListQuery{
		MeterName:  "api_calls",
		From:       time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		BucketSize: domainusage.BucketDay,
	})
	if err != nil {
		t.Fatalf("list all subject usage: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Subject != "" || buckets[0].Quantity != 10 {
		t.Fatalf("all subject buckets = %#v, want one unscoped bucket with quantity 10", buckets)
	}

	grouped, err := service.List(ctx, ListQuery{
		MeterName:  "api_calls",
		From:       time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		BucketSize: domainusage.BucketDay,
		GroupBy:    []string{domainusage.GroupBySubject},
	})
	if err != nil {
		t.Fatalf("list grouped subject usage: %v", err)
	}
	if len(grouped) != 2 {
		t.Fatalf("grouped bucket count = %d, want 2: %#v", len(grouped), grouped)
	}
	if grouped[0].Subject != "org_123" || grouped[0].Group[domainusage.GroupBySubject] != "org_123" || grouped[0].Quantity != 5 {
		t.Fatalf("first grouped subject bucket = %#v", grouped[0])
	}
	if grouped[1].Subject != "org_456" || grouped[1].Group[domainusage.GroupBySubject] != "org_456" || grouped[1].Quantity != 5 {
		t.Fatalf("second grouped subject bucket = %#v", grouped[1])
	}
}

func TestServiceListDimensionValues(t *testing.T) {
	ctx := context.Background()
	store, meterRepo, usageRepo := newTestRepositories(t, ctx)

	meter, err := domainmeter.New(
		"meter-1",
		"api_calls",
		"API calls",
		"call",
		domainmeter.AggregationSum,
		map[string]domainmeter.MetadataType{
			"region": domainmeter.MetadataString,
		},
		0,
		time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("new meter: %v", err)
	}
	if _, err := meterRepo.Save(ctx, meter); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	service := NewService(meterRepo, usageRepo, store)
	for _, event := range []CreateCommand{
		{Subject: "org_123", MeterName: "api_calls", Quantity: 2, EventTime: time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC), Metadata: map[string]any{"region": "us-east-1"}},
		{Subject: "org_123", MeterName: "api_calls", Quantity: 3, EventTime: time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC), Metadata: map[string]any{"region": "us-west-2"}},
		{Subject: "org_123", MeterName: "api_calls", Quantity: 5, EventTime: time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC), Metadata: map[string]any{"region": "us-east-1"}},
		{Subject: "org_456", MeterName: "api_calls", Quantity: 7, EventTime: time.Date(2026, 6, 8, 13, 0, 0, 0, time.UTC), Metadata: map[string]any{"region": "us-central-1"}},
	} {
		if _, err := service.Create(ctx, event); err != nil {
			t.Fatalf("create usage: %v", err)
		}
	}

	values, err := service.ListDimensionValues(ctx, DimensionValueListQuery{
		MeterName: "api_calls",
		Field:     "region",
		Subject:   "org_123",
		From:      time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		To:        time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("list dimension values: %v", err)
	}
	if len(values.Items) != 2 {
		t.Fatalf("dimension values = %#v, want two values", values.Items)
	}
	if values.Items[0].Field != "region" || values.Items[0].Value != "us-east-1" || values.Items[0].UsageEvents != 2 {
		t.Fatalf("first dimension value = %#v", values.Items[0])
	}
	if values.Items[1].Value != "us-west-2" || values.Items[1].UsageEvents != 1 {
		t.Fatalf("second dimension value = %#v", values.Items[1])
	}
}

func TestServiceListDimensionValuesRejectsUnknownField(t *testing.T) {
	ctx := context.Background()
	service := newTestService(t, ctx)

	_, err := service.ListDimensionValues(ctx, DimensionValueListQuery{
		MeterName: "api_calls",
		Field:     "region",
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("dimension field error = %v, want ErrInvalidInput", err)
	}
}

func TestServiceCreateMissingMeterReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	store, meterRepo, usageRepo := newTestRepositories(t, ctx)
	service := NewService(meterRepo, usageRepo, store)

	_, err := service.Create(ctx, CreateCommand{
		Subject:   "org_123",
		MeterName: "missing",
		Quantity:  1,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing meter error = %v, want ErrNotFound", err)
	}
}

func TestServiceListInvalidTimeRangeReturnsInvalidInput(t *testing.T) {
	ctx := context.Background()
	service := newTestService(t, ctx)

	_, err := service.List(ctx, ListQuery{
		MeterName: "api_calls",
		From:      time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		To:        time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("invalid query error = %v, want ErrInvalidInput", err)
	}
}

func TestServiceListRejectsWideHourlyRange(t *testing.T) {
	ctx := context.Background()
	service := newTestService(t, ctx)

	_, err := service.List(ctx, ListQuery{
		Subject:    "org_123",
		MeterName:  "api_calls",
		From:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC),
		BucketSize: domainusage.BucketHour,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("wide hourly query error = %v, want ErrInvalidInput", err)
	}
}

func newTestService(t *testing.T, ctx context.Context) Service {
	t.Helper()

	store, meterRepo, usageRepo := newTestRepositories(t, ctx)

	meter, err := domainmeter.New(
		"meter-1",
		"api_calls",
		"API calls",
		"call",
		domainmeter.AggregationSum,
		map[string]domainmeter.MetadataType{},
		0,
		time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("new meter: %v", err)
	}
	if _, err := meterRepo.Save(ctx, meter); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	return NewService(meterRepo, usageRepo, store)
}

func newTestRepositories(t *testing.T, ctx context.Context) (*sqlite.Store, *sqlite.MeterRepository, *sqlite.UsageRepository) {
	t.Helper()

	store, err := sqlite.NewStore(ctx, ":memory:", config.DBPoolConfig{MaxOpenConns: 1})
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}

	return store, sqlite.NewMeterRepository(store), sqlite.NewUsageRepository(store)
}
