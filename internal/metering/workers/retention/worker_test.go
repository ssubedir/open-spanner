package retention

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ssubedir/open-spanner/internal/config"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

func TestWorkerPrunesWithSQLiteRepositories(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.NewStore(ctx, ":memory:", config.DBPoolConfig{MaxOpenConns: 1})
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close sqlite store: %v", err)
		}
	})

	meterRepo := sqlite.NewMeterRepository(store)
	usageRepo := sqlite.NewUsageRepository(store)
	meter, err := domainmeter.New(
		"meter-1",
		"api_calls",
		"API calls",
		"call",
		domainmeter.AggregationSum,
		map[string]domainmeter.MetadataType{},
		1,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("new meter: %v", err)
	}
	if _, err := meterRepo.Save(ctx, meter); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	now := time.Now().UTC()
	for _, event := range []domainusage.Event{
		newUsageEvent(t, "old-event", "org_123", "api_calls", 1, now.Add(-48*time.Hour)),
		newUsageEvent(t, "new-event", "org_123", "api_calls", 2, now),
	} {
		if _, err := usageRepo.Save(ctx, event); err != nil {
			t.Fatalf("save usage event: %v", err)
		}
	}

	service := appusage.NewService(meterRepo, usageRepo, store)
	worker := NewWorker(service, 5*time.Millisecond, func(string, ...any) {})
	stop := worker.Start(ctx)
	t.Cleanup(stop)

	if !waitFor(t, 500*time.Millisecond, func() bool {
		count, err := usageRepo.CountEvents(ctx)
		if err != nil {
			t.Fatalf("count usage events: %v", err)
		}
		return count == 1
	}) {
		count, _ := usageRepo.CountEvents(ctx)
		t.Fatalf("worker did not prune old event, count=%d", count)
	}

	runs, err := usageRepo.FindPruneRuns(ctx, domainusage.NewRunQuery(10, time.Time{}, ""))
	if err != nil {
		t.Fatalf("find prune runs: %v", err)
	}
	if len(runs) == 0 || runs[0].Deleted() != 1 || runs[0].DryRun() {
		t.Fatalf("prune runs = %#v, want one real run deleting one event", runs)
	}
}

func newUsageEvent(t *testing.T, id string, subject string, meter string, quantity float64, eventTime time.Time) domainusage.Event {
	t.Helper()

	event, err := domainusage.NewEvent(
		id,
		"",
		subject,
		meter,
		quantity,
		eventTime,
		eventTime.Add(time.Second),
		map[string]any{},
	)
	if err != nil {
		t.Fatalf("new usage event: %v", err)
	}
	return event
}

func waitFor(t *testing.T, timeout time.Duration, fn func() bool) bool {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

func TestWorkerPrunesOnIntervalAndStops(t *testing.T) {
	pruner := &fakePruner{}
	worker := NewWorker(pruner, 5*time.Millisecond, func(string, ...any) {})

	stop := worker.Start(context.Background())
	if !pruner.waitForCalls(1, 200*time.Millisecond) {
		t.Fatal("worker did not prune on interval")
	}

	stop()
	callsAfterStop := pruner.calls()
	time.Sleep(20 * time.Millisecond)
	if pruner.calls() != callsAfterStop {
		t.Fatalf("worker continued after stop: before=%d after=%d", callsAfterStop, pruner.calls())
	}
}

type fakePruner struct {
	mu    sync.Mutex
	count int
}

func (p *fakePruner) PruneEvents(ctx context.Context, cmd appusage.PruneCommand) (appusage.PruneResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.count++
	return appusage.PruneResult{ID: "run-1"}, nil
}

func (p *fakePruner) calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.count
}

func (p *fakePruner) waitForCalls(want int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if p.calls() >= want {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}
