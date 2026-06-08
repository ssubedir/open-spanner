package retention

import (
	"context"
	"sync"
	"testing"
	"time"

	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
)

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
