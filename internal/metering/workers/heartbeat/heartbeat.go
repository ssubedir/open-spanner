package heartbeat

import (
	"context"
	"time"
)

type Recorder interface {
	RecordWorkerHeartbeat(ctx context.Context, workerName string, startedAt, heartbeatAt time.Time) error
}

func Start(ctx context.Context, recorder Recorder, workerName string, logger func(string, ...any)) {
	startedAt := time.Now().UTC()
	record := func() {
		if err := recorder.RecordWorkerHeartbeat(ctx, workerName, startedAt, time.Now().UTC()); err != nil && ctx.Err() == nil {
			logger("%s worker heartbeat failed: %v", workerName, err)
		}
	}
	record()
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				record()
			}
		}
	}()
}
