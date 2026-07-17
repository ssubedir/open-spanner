package heartbeat

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type Recorder interface {
	RecordWorkerHeartbeat(ctx context.Context, workerName, instanceID string, startedAt, heartbeatAt time.Time) error
	RemoveWorkerHeartbeat(ctx context.Context, workerName, instanceID string) error
}

func Start(ctx context.Context, recorder Recorder, workerName string, logger func(string, ...any)) func() {
	workerCtx, cancel := context.WithCancel(ctx)
	startedAt := time.Now().UTC()
	instanceID := InstanceID()
	record := func() {
		if err := recorder.RecordWorkerHeartbeat(workerCtx, workerName, instanceID, startedAt, time.Now().UTC()); err != nil && workerCtx.Err() == nil {
			logger("%s worker heartbeat failed: instance_id=%s error=%v", workerName, instanceID, err)
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
			removeCtx, cancelRemove := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelRemove()
			if err := recorder.RemoveWorkerHeartbeat(removeCtx, workerName, instanceID); err != nil {
				logger("%s worker heartbeat removal failed: instance_id=%s error=%v", workerName, instanceID, err)
			}
		})
	}
}

func InstanceID() string {
	if configured := strings.TrimSpace(os.Getenv("OPEN_SPANNER_WORKER_INSTANCE_ID")); configured != "" {
		return configured
	}
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "process"
	}
	return fmt.Sprintf("%s-%d", hostname, os.Getpid())
}
