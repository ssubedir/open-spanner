package heartbeat

import (
	"context"
	"sync"
	"time"
)

type Recorder interface {
	RecordWorkerHeartbeat(ctx context.Context, workerName string, startedAt, heartbeatAt time.Time) error
}

func Start(ctx context.Context, recorder Recorder, workerName string, logger func(string, ...any)) func() {
	workerCtx, cancel := context.WithCancel(ctx)
	startedAt := time.Now().UTC()
	record := func() {
		if err := recorder.RecordWorkerHeartbeat(workerCtx, workerName, startedAt, time.Now().UTC()); err != nil && workerCtx.Err() == nil {
			logger("%s worker heartbeat failed: %v", workerName, err)
		}
	}
	record()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				record()
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
}
