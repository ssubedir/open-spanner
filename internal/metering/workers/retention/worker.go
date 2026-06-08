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
	logger   Logger
}

func NewWorker(pruner Pruner, interval time.Duration, logger Logger) *Worker {
	if logger == nil {
		logger = log.Printf
	}
	return &Worker{
		pruner:   pruner,
		interval: interval,
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

	w.logger("retention prune worker started: interval=%s", w.interval)
	defer w.logger("retention prune worker stopped")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := w.pruner.PruneEvents(ctx, appusage.PruneCommand{})
			if err != nil {
				w.logger("retention prune failed: %v", err)
				continue
			}
			w.logger("retention prune completed: deleted=%d run_id=%s", result.Deleted, result.ID)
		}
	}
}
