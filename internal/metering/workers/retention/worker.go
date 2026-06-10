package retention

import (
	"context"
	"log"
	"sync"
	"time"

	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
)

type Pruner interface {
	PruneEvents(ctx context.Context, cmd appusage.PruneCommand) (appusage.PruneResult, error)
}

type Logger func(format string, args ...any)

type Worker struct {
	pruner   Pruner
	interval time.Duration
	timeout  time.Duration
	logger   Logger
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

	type pruneResult struct {
		result   appusage.PruneResult
		duration time.Duration
		err      error
	}

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

			startedAt := time.Now()
			result, err := w.pruner.PruneEvents(runCtx, appusage.PruneCommand{})
			finished <- pruneResult{
				result:   result,
				duration: time.Since(startedAt),
				err:      err,
			}
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
			w.logger("retention prune completed: duration=%s deleted=%d run_id=%s", result.duration.Round(time.Millisecond), result.result.Deleted, result.result.ID)
		case <-ticker.C:
			if running {
				w.logger("retention prune skipped: previous run still active")
				continue
			}
			startPrune()
		}
	}
}
