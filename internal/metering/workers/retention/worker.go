package retention

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	appconsumption "github.com/ssubedir/open-spanner/internal/metering/app/consumption"
	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
)

type Pruner interface {
	PruneEvents(ctx context.Context, cmd appusage.PruneCommand) (appusage.PruneResult, error)
}

type DecisionPruner interface {
	PruneDecisions(ctx context.Context, cmd appconsumption.PruneCommand) (appconsumption.PruneResult, error)
}

type WorkspaceLister interface {
	ListWorkspaceIDs(ctx context.Context) ([]string, error)
}

type LeaseCoordinator interface {
	ClaimMaintenanceLease(context.Context, string, time.Time, time.Time) (appsystem.MaintenanceLease, bool, error)
	ReleaseMaintenanceLease(context.Context, appsystem.MaintenanceLease) error
}

type Logger func(format string, args ...any)

type Worker struct {
	pruner            Pruner
	interval          time.Duration
	timeout           time.Duration
	logger            Logger
	decisionPruner    DecisionPruner
	decisionRetention time.Duration
	workspaceLister   WorkspaceLister
	leaseCoordinator  LeaseCoordinator
}

type pruneResult struct {
	eventsDeleted    int
	decisionsDeleted int
	workspaces       int
	duration         time.Duration
	err              error
}

func (w *Worker) WithDecisionPruner(pruner DecisionPruner, retention time.Duration) *Worker {
	w.decisionPruner = pruner
	w.decisionRetention = retention
	return w
}

func (w *Worker) WithWorkspaceLister(lister WorkspaceLister) *Worker {
	w.workspaceLister = lister
	return w
}

func (w *Worker) WithLeaseCoordinator(coordinator LeaseCoordinator) *Worker {
	w.leaseCoordinator = coordinator
	return w
}

func NewWorker(pruner Pruner, interval time.Duration, timeout time.Duration, logger Logger) *Worker {
	if logger == nil {
		logger = log.Printf
	}
	return &Worker{
		pruner:   pruner,
		interval: interval,
		timeout:  timeout,
		logger:   logger,
	}
}

func (w *Worker) Start(ctx context.Context) func() {
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		w.run(workerCtx)
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
}

func (w *Worker) run(ctx context.Context) {
	if w.pruner == nil || w.interval <= 0 {
		return
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.logger("retention prune worker started: interval=%s timeout=%s", w.interval, w.timeout)
	defer w.logger("retention prune worker stopped")

	finished := make(chan pruneResult, 1)
	var wg sync.WaitGroup
	running := false

	startPrune := func() {
		running = true
		wg.Add(1)
		go func() {
			defer wg.Done()

			runCtx := ctx
			cancel := func() {}
			if w.timeout > 0 {
				runCtx, cancel = context.WithTimeout(ctx, w.timeout)
			}
			defer cancel()

			finished <- w.prune(runCtx)
		}()
	}

	defer wg.Wait()

	for {
		select {
		case <-ctx.Done():
			return
		case result := <-finished:
			running = false
			if result.err != nil {
				w.logger("retention prune failed: duration=%s error=%v", result.duration.Round(time.Millisecond), result.err)
				continue
			}
			w.logger("retention prune completed: duration=%s workspaces=%d events_deleted=%d decisions_deleted=%d", result.duration.Round(time.Millisecond), result.workspaces, result.eventsDeleted, result.decisionsDeleted)
		case <-ticker.C:
			if running {
				w.logger("retention prune skipped: previous run still active")
				continue
			}
			startPrune()
		}
	}
}

func (w *Worker) prune(ctx context.Context) pruneResult {
	startedAt := time.Now()
	if w.leaseCoordinator != nil {
		now := time.Now().UTC()
		leaseTTL := 15 * time.Minute
		if w.timeout > 0 {
			leaseTTL = w.timeout + 30*time.Second
		}
		lease, claimed, err := w.leaseCoordinator.ClaimMaintenanceLease(ctx, "retention", now, now.Add(leaseTTL))
		if err != nil {
			return pruneResult{duration: time.Since(startedAt), err: err}
		}
		if !claimed {
			return pruneResult{duration: time.Since(startedAt)}
		}
		defer func() { _ = w.leaseCoordinator.ReleaseMaintenanceLease(context.WithoutCancel(ctx), lease) }()
	}
	workspaceIDs := []string{""}
	if w.workspaceLister != nil {
		var err error
		workspaceIDs, err = w.workspaceLister.ListWorkspaceIDs(ctx)
		if err != nil {
			return pruneResult{duration: time.Since(startedAt), err: err}
		}
	}

	result := pruneResult{workspaces: len(workspaceIDs)}
	for _, workspaceID := range workspaceIDs {
		workspaceCtx := ctx
		if workspaceID != "" {
			workspaceCtx = appauth.WithWorkspaceID(ctx, workspaceID)
		}
		pruned, err := w.pruner.PruneEvents(workspaceCtx, appusage.PruneCommand{})
		if err != nil {
			result.err = errors.Join(result.err, fmt.Errorf("workspace %s: %w", workspaceID, err))
			continue
		}
		result.eventsDeleted += pruned.Deleted
		if w.decisionPruner != nil && w.decisionRetention > 0 {
			decisions, err := w.decisionPruner.PruneDecisions(workspaceCtx, appconsumption.PruneCommand{Before: time.Now().UTC().Add(-w.decisionRetention)})
			if err != nil {
				result.err = errors.Join(result.err, fmt.Errorf("workspace %s decisions: %w", workspaceID, err))
				continue
			}
			result.decisionsDeleted += decisions.Deleted
		}
	}
	result.duration = time.Since(startedAt)
	return result
}
